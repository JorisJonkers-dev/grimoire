import { setDiceSetImage } from '@/infrastructure/api/sdk.gen'

/** Uploads raw bytes; the generated validator types binary bodies as strings, so it is skipped. */
export async function uploadSetPicture(diceSetId: string, file: Blob): Promise<void> {
  const body = (await file.arrayBuffer()) as unknown as Blob
  await setDiceSetImage({ path: { diceSetId }, body, requestValidator: undefined, throwOnError: true })
}
