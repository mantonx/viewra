/**
 * Convert a string to a URL-friendly slug
 *
 * @param text - The text to slugify
 * @returns URL-safe slug
 *
 * @example
 * slugify("The Beatles") // "the-beatles"
 * slugify("1923") // "1923"
 * slugify("Anthony Bourdain: Parts Unknown") // "anthony-bourdain-parts-unknown"
 */
export const slugify = (text: string): string => {
  return (
    text
      .toLowerCase()
      .trim()
      // Replace spaces with hyphens
      .replace(/\s+/g, '-')
      // Remove special characters except hyphens
      .replace(/[^\w-]+/g, '')
      // Replace multiple hyphens with single hyphen
      .replace(/--+/g, '-')
      // Remove leading/trailing hyphens
      .replace(/^-+|-+$/g, '')
  )
}
