// Test localeCompare to understand the behavior

const titleA = "affliction"
const titleB = "skin i live in"

console.log('titleA:', titleA)
console.log('titleB:', titleB)
console.log()

// For A-Z (ascending): titleA.localeCompare(titleB)
const ascResult = titleA.localeCompare(titleB, undefined, { numeric: true, sensitivity: 'base' })
console.log('titleA.localeCompare(titleB) =', ascResult)
console.log('  This means:', ascResult < 0 ? 'titleA comes BEFORE titleB (A < S) ✓ A-Z' : 'titleA comes AFTER titleB')

console.log()

// For Z-A (descending): titleB.localeCompare(titleA)
const descResult = titleB.localeCompare(titleA, undefined, { numeric: true, sensitivity: 'base' })
console.log('titleB.localeCompare(titleA) =', descResult)
console.log('  This means:', descResult < 0 ? 'titleB comes BEFORE titleA (S < A) - WRONG!' : 'titleB comes AFTER titleA (S > A) ✓ Z-A')

console.log()
console.log('=== Array Sort Test ===')
const movies = [
  { title: "Affliction" },
  { title: "The Skin I Live In" },
  { title: "Reality Bites" },
  { title: "Zombieland" }
]

const normalizeTitle = (title) => {
  return title.toLowerCase().trim().replace(/^(the|a|an)\s+/i, '')
}

// Test current logic (title-desc)
const sorted = [...movies].sort((a, b) => {
  const titleA = normalizeTitle(a.title)
  const titleB = normalizeTitle(b.title)
  return titleB.localeCompare(titleA, undefined, { numeric: true, sensitivity: 'base' })
})

console.log('Sorted with titleB.localeCompare(titleA):')
sorted.forEach((m, i) => console.log(`  ${i + 1}. ${m.title} (${normalizeTitle(m.title)})`))
