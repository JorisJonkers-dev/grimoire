import { clearTokenIcon, setPortrait, setTokenIcon } from '@/infrastructure/api/sdk.gen'

export const maxPictureBytes = 10 * 1024 * 1024
export const pictureTypes = ['image/png', 'image/jpeg', 'image/webp']

export type PictureKind = 'portrait' | 'token'
export type Ids = { campaignId: string; characterId: string }

/** Why a file cannot be uploaded, or null when it can. */
export function pictureProblem(file: Blob): string | null {
  if (!pictureTypes.includes(file.type)) return 'Pictures must be PNG, JPEG or WebP.'
  if (file.size > maxPictureBytes) return 'Pictures must be at most 10 MB.'
  return null
}

/** Uploads raw bytes; the generated validator types binary bodies as strings, so it is skipped. */
export async function uploadPicture(kind: PictureKind, path: Ids, file: Blob): Promise<void> {
  const body = (await file.arrayBuffer()) as unknown as Blob
  const options = { path, body, requestValidator: undefined, throwOnError: true as const }
  if (kind === 'portrait') await setPortrait(options)
  else await setTokenIcon(options)
}

export async function showInitials(path: Ids): Promise<void> {
  await clearTokenIcon({ path, throwOnError: true })
}

export type Crop = { zoom: number; x: number; y: number }

/** The square of the source image a token icon shows: zoom 1 is the largest square, x and y pan from -1 to 1. */
export function cropRect(width: number, height: number, crop: Crop): { sx: number; sy: number; size: number } {
  const size = Math.min(width, height) / Math.max(1, crop.zoom)
  const clamp = (v: number) => Math.min(1, Math.max(-1, v))
  const sx = ((width - size) / 2) * (1 + clamp(crop.x))
  const sy = ((height - size) / 2) * (1 + clamp(crop.y))
  return { sx, sy, size }
}

/** Draws the crop to a square PNG, or null where the browser has no canvas. */
export async function cropToBlob(image: CanvasImageSource & { naturalWidth: number; naturalHeight: number }, crop: Crop, side = 256): Promise<Blob | null> {
  const canvas = document.createElement('canvas')
  canvas.width = side
  canvas.height = side
  const ctx = canvas.getContext('2d')
  if (!ctx) return null
  const { sx, sy, size } = cropRect(image.naturalWidth, image.naturalHeight, crop)
  ctx.drawImage(image, sx, sy, size, size, 0, 0, side, side)
  return new Promise((resolve) => {
    canvas.toBlob((blob) => {
      resolve(blob)
    }, 'image/png')
  })
}
