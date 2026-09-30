import { afterEach, describe, expect, it, vi } from 'vitest'
import { configureApi } from '@/infrastructure/http'
import { cropRect, cropToBlob, pictureProblem, showInitials, uploadPicture } from './pictures'

const ids = { campaignId: '0190c7a8-0000-7000-8000-000000000001', characterId: '0190c7a8-0000-7000-8000-000000000009' }

afterEach(() => {
  vi.restoreAllMocks()
})

describe('pictures', () => {
  it('accepts only small PNG, JPEG and WebP files', () => {
    expect(pictureProblem(new Blob(['x'], { type: 'image/png' }))).toBeNull()
    expect(pictureProblem(new Blob(['x'], { type: 'image/gif' }))).toContain('PNG, JPEG or WebP')
    const huge = { type: 'image/webp', size: 11 * 1024 * 1024 } as Blob
    expect(pictureProblem(huge)).toContain('10 MB')
  })

  it('crops a centred, zoomed and panned square', () => {
    expect(cropRect(400, 200, { zoom: 1, x: 0, y: 0 })).toEqual({ sx: 100, sy: 0, size: 200 })
    expect(cropRect(400, 200, { zoom: 2, x: 1, y: -1 })).toEqual({ sx: 300, sy: 0, size: 100 })
    expect(cropRect(200, 200, { zoom: 0.5, x: 5, y: 0 })).toEqual({ sx: 0, sy: 0, size: 200 })
  })

  it('uploads raw bytes and clears the token', async () => {
    const seen: Request[] = []
    const fetchImpl = vi.fn<typeof fetch>((input) => {
      seen.push(input as Request)
      return Promise.resolve(new Response(null, { status: 204 }))
    })
    configureApi({ baseUrl: 'http://localhost', fetch: fetchImpl })
    const blob = new Blob(['png'], { type: 'image/png' })
    await uploadPicture('portrait', ids, blob)
    await uploadPicture('token', ids, blob)
    await showInitials(ids)
    expect(seen.map((r) => `${r.method} ${new URL(r.url).pathname.split('/').at(-1) ?? ''}`)).toEqual(['PUT portrait', 'PUT token', 'DELETE token'])
    expect(seen[0]?.headers.get('Content-Type')).toBe('application/octet-stream')
    expect(await seen[0]?.text()).toBe('png')
  })

  it('draws the crop to a PNG, or gives up without a canvas', async () => {
    const image = { naturalWidth: 100, naturalHeight: 50 } as HTMLImageElement
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
    expect(await cropToBlob(image, { zoom: 1, x: 0, y: 0 })).toBeNull()
    const drawImage = vi.fn()
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ drawImage } as unknown as CanvasRenderingContext2D)
    vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation((cb) => {
      cb(new Blob(['x'], { type: 'image/png' }))
    })
    const blob = await cropToBlob(image, { zoom: 1, x: 0, y: 0 }, 64)
    expect(blob?.type).toBe('image/png')
    expect(drawImage).toHaveBeenCalledWith(image, 25, 0, 50, 50, 0, 0, 64, 64)
  })
})
