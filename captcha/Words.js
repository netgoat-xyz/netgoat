import readline from 'readline'

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
})

const separators = ['-', '_', '|', '~']
const noiseChars = 'abcdefghijklmnopqrstuvwxyz0123456789'

const shuffleArray = (arr) => {
  const array = [...arr]
  for (let i = array.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[array[i], array[j]] = [array[j], array[i]]
  }
  return array
}

// Generate random noise string of length n
const randomNoise = (n) =>
  Array.from({ length: n }, () =>
    noiseChars[Math.floor(Math.random() * noiseChars.length)]
  ).join('')

// Inject noise chunks between letters based on sus
const jumbleWithNoise = (word, sus) => {
  const chars = word.split('')
  const shuffled = shuffleArray(chars)

  // Noise length depends on sus, clamp to 3-6
  const noiseLen = Math.min(6, 3 + sus)
  
  // For each letter, randomly decide to add noise chunk after it (skip last letter)
  return shuffled
    .map((c, i) => {
      let chunk = c
      if (i !== shuffled.length - 1) {
        // 50% chance to add noise chunk
        if (Math.random() < 0.5) {
          chunk += randomNoise(noiseLen)
        }
        // always add a separator after the chunk
        chunk += separators[Math.floor(Math.random() * separators.length)]
      }
      return chunk
    })
    .join('')
    .replace(/[-_|~]$/, '')
}

const wordList = [
  'sunshine', 'puppy', 'icecream', 'hug', 'birthday', 'victory', 'cake',
  'payday', 'vacation', 'wifi', 'cozy', 'laugh', 'yay', 'flowers',
  'fireworks', 'snuggle', 'chill', 'dance', 'smile', 'celebration',
  'friendship', 'happiness', 'butterfly',
  'death', 'funeral', 'grief', 'loss', 'betrayal', 'divorce', 'war',
  'bankrupt', 'alone', 'ignored', 'ghosted', 'shit', 'fuck', 'bitch',
  'asshole', 'wtf', 'damn', 'cringe', 'fucked', 'broke', 'hangover',
  'tears', 'regret', 'pain', 'stabbed', 'darkness', 'crying',
  'depression', 'heartbreak', 'loneliness', 'melancholy',
  'table', 'keyboard', 'monitor', 'pen', 'lamp', 'floor', 'door',
  'fan', 'cable', 'folder', 'battery', 'shelf', 'chair', 'box', 'book',
  'paper', 'spoon', 'window', 'tv', 'mug', 'phone', 'router', 'towel',
  'notebook', 'calculator', 'backpack', 'headphones',
  'blorn', 'gretz', 'morbix', 'twazzle', 'crundle', 'viblor', 'zontra',
  'narpin', 'klirb', 'dritz', 'skarn', 'dribblet', 'snorble', 'ploob',
  'wibjot', 'gleep', 'murbit', 'flimp', 'scramblo', 'dankle'
]

const ask = (q) => new Promise((res) => rl.question(q, res))

const runCaptcha = async () => {
  let sus = 0
  const picks = shuffleArray(wordList).slice(0, 8)
  let correct = 0

  console.log('\n🧩 Unscramble the jumbled letters and type the original word.')
  console.log('Ignore dashes/underscores/etc and random noise letters.\n')

  for (const word of picks) {
    const jumbled = jumbleWithNoise(word, sus)
    const answer = await ask(`Jumbled: ${jumbled}\nYour guess: `)
    if (answer.trim().toLowerCase() === word.toLowerCase()) {
      console.log('✅ Correct!\n')
      correct++
      sus = Math.max(0, sus - 1) // chill out if right
    } else {
      console.log(`❌ Nope! The word was "${word}"\n`)
      sus++
    }
  }

  rl.close()
  console.log(`🎯 Final Score: ${correct}/${picks.length}\n`)

  if (correct === picks.length) {
    console.log('🧠 GIGA HUMAN. You crushed it.')
  } else if (correct >= Math.floor(picks.length * 0.6)) {
    console.log('🙂 Close enough. Definitely human energy.')
  } else {
    console.log('🤖 Bot vibes. You tried tho.')
  }
}

runCaptcha()
