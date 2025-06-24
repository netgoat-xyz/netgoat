import readline from 'readline'

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
})

const emotionWords = {
  happy: [
    "sunshine", "puppy", "ice cream", "hug", "birthday", "victory", "cake",
    "payday", "vacation", "wifi", "cozy", "laugh", "yay", "flowers",
    "fireworks", "snuggle", "chill", "dance", "smile"
  ],
  sad: [
    "death", "funeral", "grief", "loss", "betrayal", "divorce", "war",
    "bankrupt", "alone", "ignored", "ghosted", "shit", "fuck", "bitch",
    "asshole", "wtf", "damn", "cringe", "fucked", "broke", "hangover",
    "tears", "regret", "pain", "stabbed", "darkness", "crying"
  ],
  neutral: [
    "table", "keyboard", "monitor", "pen", "lamp", "floor", "door",
    "fan", "cable", "folder", "battery", "shelf", "chair", "box", "book",
    "paper", "spoon", "window", "tv", "mug", "phone", "router", "towel"
  ],
  fake: [
    "blorn", "gretz", "morbix", "twazzle", "crundle", "viblor", "zontra",
    "narpin", "klirb", "dritz", "skarn", "dribblet", "snorble", "ploob",
    "wibjot", "gleep", "murbit", "flimp", "scramblo", "dankle"
  ]
}

const shuffle = (arr) => [...arr].sort(() => Math.random() - 0.5)
const ask = (q) => new Promise((res) => rl.question(q, res))

const buildWordBank = () => {
  const legit = Object.entries(emotionWords)
    .filter(([emotion]) => emotion !== 'fake')
    .flatMap(([emotion, words]) => words.map(word => ({ word, emotion })))
  
  const fakes = emotionWords.fake.map(word => ({ word, emotion: 'fake' }))
  return shuffle([...legit, ...fakes])
}

const runCaptcha = async () => {
  const wordBank = buildWordBank()
  const picks = shuffle(wordBank).slice(0, 10)
  let correct = 0

  console.log('\n🧠 Match the word to the correct emotion: happy, sad, or neutral')
  console.log('🤫 Some words are fake gibberish. Just skip those (press Enter).\n')

  for (const item of picks) {
    const answer = await ask(`Word: ${item.word}\nEmotion (or skip): `)
    const cleaned = answer.trim().toLowerCase()

    if (item.emotion === 'fake') {
      if (!cleaned) {
        console.log('Good call - fake word skipped.\n')
        correct++
      } else {
        console.log('Nope. That wasnt a real emotion word.\n')
      }
    } else {
      if (cleaned === item.emotion) {
        console.log('Correct!\n')
        correct++
      } else {
        console.log(`Wrong!"\n`)
      }
    }
  }

  rl.close()
  console.log(`Final Score: ${correct}/${picks.length}\n`)

  if (correct === picks.length) {
    console.log('0% bot')
  } else if (correct >= Math.floor(picks.length * 0.7)) {
    console.log('You pass')
  } else {
    console.log('failed the CAPTCHA')
  }
}

runCaptcha()
