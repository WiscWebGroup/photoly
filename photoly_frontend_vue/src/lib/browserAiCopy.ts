import type { AiModelKind } from './ai'
import type { LoadSource } from './browserAi'

const mb = (n: number) => `${Math.round(n)}`

/** Wording for the loading card and the upload row, from the design (download vs this device). */
export function loadingCopy(
  source: LoadSource,
  loadedMB: number,
  totalMB: number,
  kind: AiModelKind | null = 'text',
) {
  const name = kind === 'image' ? 'image model' : 'search model'
  const amount = totalMB ? `${mb(loadedMB)} of ${mb(totalMB)} MB` : ''
  if (source === 'download')
    return {
      headline: `Downloading the ${name}`,
      amount,
      detail: 'This happens once. Photoly keeps the model here for next time.',
    }
  return {
    headline: source === 'device' ? `Loading the ${name} from this device` : `Preparing the ${name}`,
    amount,
    detail:
      source === 'device'
        ? 'Nothing is downloaded. The first search after opening Photoly takes a few seconds; the rest are instant.'
        : 'Checking what is already saved on this device.',
  }
}

export interface ModelSizes {
  image: number
  textSmall: number
  textFull: number
}
/** Download sizes in MB; the server's numbers win over the design defaults. */
export function modelSizes(from?: Partial<ModelSizes>): ModelSizes {
  return { image: 372, textSmall: 318, textFull: 1130, ...from }
}
export const sizeLabel = (mbSize: number) =>
  mbSize >= 1000 ? `${(mbSize / 1000).toFixed(2)} GB` : `${mbSize} MB`
