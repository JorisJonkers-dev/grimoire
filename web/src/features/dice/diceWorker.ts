/// <reference lib="webworker" />
// The GPU dice (ADR-0012): real 3D dice drawn with WebGL on an offscreen canvas, in a worker so the
// page never waits on them. Each die is thrown in from the screen's edge, tumbles and bounces, and
// settles with the server's number facing the camera; then the dice glide into a row at the centre.
// The page shows the total itself, so the result is right whatever happens here.
import {
  AmbientLight, BoxGeometry, BufferGeometry, CanvasTexture, Color, DirectionalLight, DodecahedronGeometry, Float32BufferAttribute, Group, IcosahedronGeometry,
  Mesh, MeshBasicMaterial, MeshStandardMaterial, OctahedronGeometry, OrthographicCamera, PlaneGeometry, Quaternion, SRGBColorSpace, Scene, TetrahedronGeometry,
  Vector3, WebGLRenderer,
} from 'three'
import type { DieLook, LiveDiceLook } from '@/infrastructure/api/types.gen'
import { GATHER_MS, TABLE_HEIGHT, THROW_MS, throwPlan, type DieThrow, type ShownDie } from './choreography'
import { cellOf, lookFor, sheetCols } from './sets'

export type ToWorker =
  | { type: 'init'; canvas: OffscreenCanvas; width: number; height: number; dpr: number }
  | { type: 'size'; width: number; height: number; dpr: number }
  | { type: 'roll'; dice: ShownDie[]; seed: number; look?: LiveDiceLook }
  | { type: 'clear' }
export type FromWorker = { type: 'ready' } | { type: 'failed' } | { type: 'slow' } | { type: 'settled' }

/** A device that cannot hold this many milliseconds a frame through a throw gets the 2D dice instead. */
const SLOW_FRAME_MS = 50
const SAMPLE_FRAMES = 18
const RADIUS = 1
const UP = new Vector3(0, 0, 1)

const post = (m: FromWorker) => { (self as DedicatedWorkerGlobalScope).postMessage(m) }

type Face = { centre: Vector3; normal: Vector3 }

// A d10: two rings of five vertices between two poles, each face a kite.
function trapezohedron(): BufferGeometry {
  const ring = (z: number, offset: number) =>
    Array.from({ length: 5 }, (_, i) => new Vector3(Math.cos(((i + offset) * 2 * Math.PI) / 5) * RADIUS, Math.sin(((i + offset) * 2 * Math.PI) / 5) * RADIUS, z))
  const [top, bottom, upper, lower] = [new Vector3(0, 0, RADIUS * 1.15), new Vector3(0, 0, -RADIUS * 1.15), ring(0.12, 0), ring(-0.12, 0.5)]
  const points: number[] = []
  const tri = (a: Vector3, b: Vector3, c: Vector3) => points.push(a.x, a.y, a.z, b.x, b.y, b.z, c.x, c.y, c.z)
  for (let i = 0; i < 5; i++) {
    const [u0, u1, l0, l1] = [upper[i], upper[(i + 1) % 5], lower[i], lower[(i + 4) % 5]]
    if (!u0 || !u1 || !l0 || !l1) continue
    tri(top, u0, l0)
    tri(top, l0, u1)
    tri(bottom, l0, u0)
    tri(bottom, u0, l1)
  }
  const g = new BufferGeometry()
  g.setAttribute('position', new Float32BufferAttribute(points, 3))
  g.computeVertexNormals()
  return g
}

function shapeOf(faces: number): BufferGeometry {
  switch (faces) {
    case 4:
      return new TetrahedronGeometry(RADIUS * 1.15)
    case 8:
      return new OctahedronGeometry(RADIUS * 1.1)
    case 10:
    case 100:
      return trapezohedron()
    case 12:
      return new DodecahedronGeometry(RADIUS)
    case 20:
      return new IcosahedronGeometry(RADIUS * 1.05)
    default:
      return new BoxGeometry(RADIUS * 1.5, RADIUS * 1.5, RADIUS * 1.5)
  }
}

// The flat faces of a solid: its triangles gathered by the way they face.
function facesOf(geometry: BufferGeometry): Face[] {
  const flat = geometry.index ? geometry.toNonIndexed() : geometry
  const p = flat.getAttribute('position')
  const found: { normal: Vector3; sum: Vector3; n: number }[] = []
  for (let i = 0; i < p.count; i += 3) {
    const [a, b, c] = [new Vector3().fromBufferAttribute(p, i), new Vector3().fromBufferAttribute(p, i + 1), new Vector3().fromBufferAttribute(p, i + 2)]
    const normal = new Vector3().subVectors(b, a).cross(new Vector3().subVectors(c, a)).normalize()
    const face = found.find((f) => f.normal.dot(normal) > 0.98) ?? found[found.push({ normal, sum: new Vector3(), n: 0 }) - 1]
    if (!face) continue
    face.sum.add(a).add(b).add(c)
    face.n += 3
  }
  return found.map((f) => ({ normal: f.normal, centre: f.sum.divideScalar(f.n) }))
}

function numberTexture(text: string, colour: string): CanvasTexture<OffscreenCanvas> {
  const canvas = new OffscreenCanvas(128, 128)
  const pen = canvas.getContext('2d')
  if (pen) {
    pen.fillStyle = colour
    pen.font = `bold ${text.length > 2 ? '56' : '78'}px serif`
    pen.textAlign = 'center'
    pen.textBaseline = 'middle'
    pen.fillText(text, 64, 68)
  }
  return new CanvasTexture(canvas)
}

/** A die's sheet is painted this many pixels square; every face is cut from its own cell of it. */
const SHEET_PX = 512
/** How far a face reaches from its centre at most, so that any face fits its cell. */
const FACE_REACH = RADIUS * 1.3
const DIM = 0.45

// Gives every face its own cell of the sheet, the way the Dice Set editor draws it: the face that shows
// a number is cut from the cell of that number.
function unwrap(geometry: BufferGeometry, faces: Face[], first: number): BufferGeometry {
  const flat = geometry.index ? geometry.toNonIndexed() : geometry
  const p = flat.getAttribute('position')
  const cols = sheetCols(faces.length)
  const frames = faces.map((f) => {
    const turn = new Quaternion().setFromUnitVectors(UP, f.normal)
    return { u: new Vector3(1, 0, 0).applyQuaternion(turn), v: new Vector3(0, 1, 0).applyQuaternion(turn) }
  })
  const uv: number[] = []
  for (let i = 0; i < p.count; i += 3) {
    const corners = [0, 1, 2].map((k) => new Vector3().fromBufferAttribute(p, i + k))
    const [a, b, c] = corners
    if (!a || !b || !c) continue
    const normal = new Vector3().subVectors(b, a).cross(new Vector3().subVectors(c, a)).normalize()
    const at = Math.max(0, faces.findIndex((f) => f.normal.dot(normal) > 0.98))
    const [face, frame] = [faces[at], frames[at]]
    if (!face || !frame) continue
    const { col, row } = cellOf((first + at) % faces.length, cols)
    for (const corner of corners) {
      const d = corner.clone().sub(face.centre)
      const [across, up] = [d.dot(frame.u) / (2 * FACE_REACH) + 0.5, d.dot(frame.v) / (2 * FACE_REACH) + 0.5]
      uv.push((col + across) / cols, 1 - (row + 1 - up) / cols)
    }
  }
  flat.setAttribute('uv', new Float32BufferAttribute(uv, 2))
  return flat
}

// A small generator, so a pattern paints the same on every screen.
function scatter(seed: number): () => number {
  let s = seed
  return () => {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0
    return s / 0x100000000
  }
}

// Paints the sheet a die's faces are cut from: the pattern in the body colour, then the set's picture
// where it was placed. The numbers are their own layer above it.
function paintSheet(canvas: OffscreenCanvas, look: DieLook, kept: boolean, picture: ImageBitmap | null) {
  const pen = canvas.getContext('2d')
  if (!pen) return
  const size = canvas.width
  const body = new Color(look.body).multiplyScalar(kept ? 1 : DIM)
  const hex = (c: Color) => `#${c.getHexString()}`
  const [light, dark] = [hex(body.clone().lerp(new Color('#ffffff'), 0.3)), hex(body.clone().lerp(new Color('#000000'), 0.4))]
  const next = scatter(7)
  pen.globalAlpha = 1
  pen.fillStyle = hex(body)
  pen.fillRect(0, 0, size, size)
  if (look.pattern === 'marble') {
    pen.strokeStyle = light
    pen.globalAlpha = 0.6
    for (let i = 0; i < 14; i++) {
      pen.lineWidth = 2 + next() * 6
      pen.beginPath()
      pen.moveTo(next() * size, 0)
      pen.bezierCurveTo(next() * size, size * 0.3, next() * size, size * 0.6, next() * size, size)
      pen.stroke()
    }
  } else if (look.pattern === 'speckled') {
    for (let i = 0; i < 900; i++) {
      pen.fillStyle = i % 2 ? light : dark
      pen.beginPath()
      pen.arc(next() * size, next() * size, 1.5 + next() * 3, 0, Math.PI * 2)
      pen.fill()
    }
  } else if (look.pattern === 'stripes') {
    pen.strokeStyle = light
    pen.lineWidth = size / 24
    for (let x = -size; x < size * 2; x += size / 10) {
      pen.beginPath()
      pen.moveTo(x, size)
      pen.lineTo(x + size, 0)
      pen.stroke()
    }
  }
  if (picture && look.image) {
    const width = look.image.scale * size
    const height = width * (picture.height / picture.width)
    pen.save()
    pen.globalAlpha = kept ? 1 : DIM
    pen.translate(look.image.x * size, look.image.y * size)
    pen.rotate((look.image.rotation * Math.PI) / 180)
    pen.drawImage(picture, -width / 2, -height / 2, width, height)
    pen.restore()
  }
}

// A set's picture is fetched once; a picture that cannot be had leaves the dice in their pattern.
const pictures = new Map<string, Promise<ImageBitmap | null>>()
function pictureAt(url: string): Promise<ImageBitmap | null> {
  const known = pictures.get(url)
  if (known) return known
  const fetched = fetch(url, { credentials: 'same-origin' })
    .then((r) => (r.ok ? r.blob() : Promise.reject(new Error('no picture'))))
    .then((b) => createImageBitmap(b))
    .catch(() => null)
  pictures.set(url, fetched)
  return fetched
}

type Die = { group: Group; plan: DieThrow; target: Quaternion; tumble: Vector3 }

// The body of a die in its Dice Set: the sheet as its skin, repainted when the set's picture arrives.
function dressed(geometry: BufferGeometry, faces: Face[], die: ShownDie, look: DieLook, imageUrl?: string): Mesh {
  const canvas = new OffscreenCanvas(SHEET_PX, SHEET_PX)
  paintSheet(canvas, look, die.kept, null)
  const map = new CanvasTexture(canvas)
  map.colorSpace = SRGBColorSpace
  if (imageUrl && look.image) {
    void pictureAt(imageUrl).then((picture) => {
      if (!picture) return
      paintSheet(canvas, look, die.kept, picture)
      map.needsUpdate = true
      if (renderer && camera) renderer.render(scene, camera)
    })
  }
  return new Mesh(unwrap(geometry, faces, die.value - 1), new MeshStandardMaterial({ map, flatShading: true, roughness: 0.55, metalness: 0.1 }))
}

// Builds a die whose first face shows the server's number; the other faces count on from it.
function build(die: ShownDie, plan: DieThrow, set?: LiveDiceLook): Die {
  const geometry = shapeOf(die.faces)
  const group = new Group()
  const faces = facesOf(geometry)
  const look = lookFor(set, die.faces)
  const plain = () => new Mesh(geometry, new MeshStandardMaterial({ color: new Color(die.kept ? '#7a1f1a' : '#3a2f2b'), flatShading: true, roughness: 0.55, metalness: 0.1 }))
  group.add(look ? dressed(geometry, faces, die, look, set?.imageUrl) : plain())
  const ink = look ? `#${new Color(look.numbers).multiplyScalar(die.kept ? 1 : 0.6).getHexString()}` : die.kept ? '#f3d27a' : '#9a8f86'
  faces.forEach((face, i) => {
    const value = ((die.value - 1 + i) % die.faces) + 1
    const label = new Mesh(new PlaneGeometry(RADIUS * 0.8, RADIUS * 0.8), new MeshBasicMaterial({ map: numberTexture(String(value), ink), transparent: true }))
    label.position.copy(face.centre).addScaledVector(face.normal, 0.012)
    label.quaternion.setFromUnitVectors(UP, face.normal)
    group.add(label)
  })
  // It lands with its first face to the camera, turned so that the number on it reads upright.
  const landing = faces[0]?.normal ?? UP
  const flat = new Quaternion().setFromUnitVectors(landing, UP)
  const top = new Vector3(0, 1, 0).applyQuaternion(new Quaternion().setFromUnitVectors(UP, landing)).applyQuaternion(flat)
  const target = new Quaternion().setFromAxisAngle(UP, Math.atan2(top.x, top.y)).multiply(flat)
  const tumble = new Vector3(Math.cos(plan.yaw), Math.sin(plan.yaw), 0.6).normalize()
  return { group, plan, target, tumble }
}

let renderer: WebGLRenderer | undefined
let camera: OrthographicCamera | undefined
const scene = new Scene()
let dice: Die[] = []
let started = 0
let frames: number[] = []
let judged = false
let frame = 0

function frameCamera(width: number, height: number, dpr: number) {
  const half = { x: (TABLE_HEIGHT * (width / height)) / 2, y: TABLE_HEIGHT / 2 }
  camera = new OrthographicCamera(-half.x, half.x, half.y, -half.y, 0.1, 100)
  camera.position.set(0, 0, 20)
  camera.lookAt(0, 0, 0)
  renderer?.setPixelRatio(Math.min(dpr, 2))
  renderer?.setSize(width, height, false)
}

const ease = (t: number) => 1 - (1 - t) ** 3

function place(d: Die, elapsed: number) {
  const t = Math.min(1, elapsed / THROW_MS)
  const g = Math.min(1, Math.max(0, (elapsed - THROW_MS) / GATHER_MS))
  const at = { x: d.plan.from.x + (d.plan.rest.x - d.plan.from.x) * ease(t), y: d.plan.from.y + (d.plan.rest.y - d.plan.from.y) * ease(t) }
  // Three bounces, each lower than the last, then flat on the table.
  const hop = Math.abs(Math.sin(t * Math.PI * 3)) * (1 - t) ** 2 * 3
  d.group.position.set(at.x + (d.plan.gather.x - d.plan.rest.x) * ease(g), at.y + (d.plan.gather.y - d.plan.rest.y) * ease(g), RADIUS + hop)
  // It tumbles freely, then turns the last of the way onto the server's face.
  const turning = new Quaternion().setFromAxisAngle(d.tumble, d.plan.spin * (1 - t) ** 2 * Math.PI)
  d.group.quaternion.copy(d.target).multiply(turning)
}

function tick(now: number) {
  if (!renderer || !camera) return
  const elapsed = now - started
  frames.push(now)
  for (const d of dice) place(d, elapsed)
  renderer.render(scene, camera)
  if (!judged && frames.length > SAMPLE_FRAMES) {
    judged = true
    const first = frames[2] ?? now
    if ((now - first) / (frames.length - 3) > SLOW_FRAME_MS) post({ type: 'slow' })
  }
  if (elapsed < THROW_MS + GATHER_MS) frame = requestAnimationFrame(tick)
  else post({ type: 'settled' })
}

function clear() {
  cancelAnimationFrame(frame)
  for (const d of dice) {
    scene.remove(d.group)
    d.group.traverse((o) => {
      if (!(o instanceof Mesh)) return
      const material = o.material as MeshStandardMaterial | MeshBasicMaterial
      ;(o.geometry as BufferGeometry).dispose()
      material.map?.dispose()
      material.dispose()
    })
  }
  dice = []
  if (renderer && camera) renderer.render(scene, camera)
}

function roll(shown: ShownDie[], seed: number, look?: LiveDiceLook) {
  if (!renderer || !camera) return
  clear()
  const aspect = (camera.right - camera.left) / (camera.top - camera.bottom)
  dice = shown.map((die, i) => build(die, throwPlan(shown, seed, aspect)[i] ?? { from: { x: 0, y: 0 }, rest: { x: 0, y: 0 }, gather: { x: 0, y: 0 }, spin: 0, yaw: 0 }, look))
  for (const d of dice) scene.add(d.group)
  started = performance.now()
  frames = []
  judged = false
  frame = requestAnimationFrame(tick)
}

self.onmessage = (e: MessageEvent<ToWorker>) => {
  const m = e.data
  switch (m.type) {
    case 'init':
      try {
        renderer = new WebGLRenderer({ canvas: m.canvas, alpha: true, antialias: true })
        scene.add(new AmbientLight(0xffffff, 1.1))
        const sun = new DirectionalLight(0xffffff, 2.2)
        sun.position.set(-4, 6, 12)
        scene.add(sun)
        frameCamera(m.width, m.height, m.dpr)
        post({ type: 'ready' })
      } catch {
        post({ type: 'failed' })
      }
      return
    case 'size':
      frameCamera(m.width, m.height, m.dpr)
      return
    case 'roll':
      roll(m.dice, m.seed, m.look)
      return
    case 'clear':
      clear()
  }
}
