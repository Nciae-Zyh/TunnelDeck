import { readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { Resvg } from '@resvg/resvg-js'

const sourceUrl = new URL('../../build/appicon.svg', import.meta.url)
const source = await readFile(sourceUrl)

const outputs = [
  [16, new URL('../public/icons/icon-16.png', import.meta.url)],
  [32, new URL('../public/icons/icon-32.png', import.meta.url)],
  [48, new URL('../public/icons/icon-48.png', import.meta.url)],
  [128, new URL('../public/icons/icon-128.png', import.meta.url)],
  [1024, new URL('../../build/appicon.png', import.meta.url)],
]

for (const [size, outputUrl] of outputs) {
  const renderer = new Resvg(source, {
    fitTo: { mode: 'width', value: size },
  })

  await writeFile(outputUrl, renderer.render().asPng())
  console.log(`Rendered ${size}px icon: ${fileURLToPath(outputUrl)}`)
}
