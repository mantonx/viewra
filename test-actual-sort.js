const movies = [
  { title: "The Skin I Live In" },
  { title: "Reality Bites (2004)" },
  { title: "Affliction" },
  { title: "3 Men and a Little Lady" },
  { title: "(500) Days of Summer" }
]

const normalizeTitle = (title) => {
  return title.toLowerCase().trim().replace(/^(the|a|an)\s+/i, '')
}

console.log('Testing title-desc (Z-A) sorting:\n')

const sorted = [...movies].sort((a, b) => {
  const titleA = normalizeTitle(a.title)
  const titleB = normalizeTitle(b.title)
  const result = titleB.localeCompare(titleA, undefined, { numeric: true, sensitivity: 'base' })
  console.log(`  Compare: "${titleB}" vs "${titleA}" = ${result} (${result < 0 ? 'B first' : result > 0 ? 'A first' : 'equal'})`)
  return result
})

console.log('\nResult:')
sorted.forEach((m, i) => console.log(`  ${i + 1}. ${m.title} (${normalizeTitle(m.title)})`))

console.log('\n\nExpected for Z-A:')
console.log('  1. The Skin I Live In (skin...)')
console.log('  2. Reality Bites (reality...)')
console.log('  3. Affliction (affliction)')
console.log('  4. 3 Men and a Little Lady (3...)')
console.log('  5. (500) Days of Summer ((...)')
