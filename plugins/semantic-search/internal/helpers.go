package internal

import (
	"strings"
)

// GenreExpansions maps user-friendly genre terms to comprehensive search terms.
// These help users find content even with inexact genre terminology.
var GenreExpansions = map[string][]string{
	// Action & Adventure
	"action":    {"action", "fight", "combat", "martial arts", "stunts", "explosive", "adrenaline"},
	"adventure": {"adventure", "quest", "journey", "exploration", "expedition", "discovery"},

	// Comedy variants
	"comedy":      {"comedy", "funny", "humor", "humorous", "comedic", "hilarious", "laughs"},
	"romcom":      {"romantic comedy", "rom-com", "romcom", "love comedy", "romantic humor"},
	"dark comedy": {"dark comedy", "black comedy", "dark humor", "satirical", "sardonic"},
	"slapstick":   {"slapstick", "physical comedy", "farce", "goofy", "silly"},

	// Drama variants
	"drama":        {"drama", "dramatic", "emotional", "character study", "serious"},
	"melodrama":    {"melodrama", "emotional drama", "tearjerker", "weepy", "sentimental"},
	"family drama": {"family drama", "domestic drama", "family conflict", "generational"},

	// Horror & Thriller
	"horror":               {"horror", "scary", "frightening", "terrifying", "creepy", "nightmare"},
	"thriller":             {"thriller", "suspense", "suspenseful", "tension", "edge of seat", "gripping"},
	"slasher":              {"slasher", "killer", "masked killer", "stalker horror", "bloody"},
	"psychological horror": {"psychological horror", "mind-bending horror", "disturbing", "unsettling"},

	// Science Fiction
	"scifi":       {"science fiction", "sci-fi", "scifi", "futuristic", "space", "technology"},
	"space opera": {"space opera", "space epic", "galactic", "interstellar", "starship"},
	"cyberpunk":   {"cyberpunk", "dystopian tech", "neon noir", "high tech low life", "hackers"},
	"dystopia":    {"dystopian", "dystopia", "post-apocalyptic", "totalitarian", "bleak future"},
	"time travel": {"time travel", "temporal", "chronological", "past and future", "timeline"},

	// Fantasy
	"fantasy":       {"fantasy", "magical", "mythical", "enchanted", "supernatural"},
	"high fantasy":  {"high fantasy", "epic fantasy", "sword and sorcery", "wizards", "elves"},
	"urban fantasy": {"urban fantasy", "modern fantasy", "magic in city", "supernatural thriller"},
	"fairy tale":    {"fairy tale", "fable", "folklore", "once upon a time", "magical story"},

	// Romance
	"romance":        {"romance", "romantic", "love story", "relationship", "passion"},
	"period romance": {"period romance", "historical romance", "costume drama romance", "regency"},

	// Documentary & Reality
	"documentary": {"documentary", "non-fiction", "real life", "factual", "docuseries"},
	"docudrama":   {"docudrama", "dramatized documentary", "based on true events", "reenactment"},
	"true crime":  {"true crime", "crime documentary", "investigation", "unsolved", "murder mystery"},

	// Animation
	"animation": {"animation", "animated", "cartoon", "anime", "CGI"},
	"anime":     {"anime", "japanese animation", "manga adaptation", "otaku"},

	// Other genres
	"western":      {"western", "cowboy", "frontier", "wild west", "gunslinger"},
	"noir":         {"noir", "film noir", "neo-noir", "hardboiled", "cynical detective"},
	"musical":      {"musical", "song and dance", "broadway", "singing", "choreography"},
	"war":          {"war", "military", "combat", "battlefield", "soldiers", "warfare"},
	"sports":       {"sports", "athletic", "competition", "team", "championship", "underdog"},
	"mystery":      {"mystery", "whodunit", "detective", "puzzle", "clues", "investigation"},
	"crime":        {"crime", "criminal", "heist", "gangster", "mafia", "organized crime"},
	"biography":    {"biography", "biopic", "biographical", "life story", "true story"},
	"historical":   {"historical", "period piece", "costume drama", "era", "past"},
	"superhero":    {"superhero", "comic book", "superpowers", "hero", "villain"},
	"martial arts": {"martial arts", "kung fu", "karate", "fighting", "combat sports"},
}

// EraExpansions maps decade/era terms to year ranges and cultural context.
var EraExpansions = map[string]struct {
	YearStart int
	YearEnd   int
	Keywords  []string
}{
	"silent era":    {1890, 1929, []string{"silent film", "black and white", "intertitles", "early cinema"}},
	"golden age":    {1930, 1959, []string{"classic hollywood", "studio system", "glamour", "old hollywood"}},
	"new hollywood": {1960, 1979, []string{"new hollywood", "auteur", "counterculture", "experimental"}},
	"80s":           {1980, 1989, []string{"eighties", "1980s", "neon", "synth", "retro 80s", "reagan era"}},
	"90s":           {1990, 1999, []string{"nineties", "1990s", "gen x", "grunge era", "clinton era"}},
	"2000s":         {2000, 2009, []string{"aughts", "2000s", "y2k", "early digital", "post-9/11"}},
	"2010s":         {2010, 2019, []string{"twenty tens", "2010s", "streaming era", "superhero era"}},
	"modern":        {2020, 2099, []string{"contemporary", "current", "recent", "new releases", "2020s"}},
	"classic":       {1920, 1969, []string{"classic", "vintage", "timeless", "old", "traditional"}},
	"retro":         {1970, 1999, []string{"retro", "nostalgic", "throwback", "vintage"}},
	"pre-code":      {1929, 1934, []string{"pre-code hollywood", "risque", "uncensored", "early talkies"}},
}

// RatingExpansions maps audience descriptors to content ratings.
var RatingExpansions = map[string][]string{
	"family friendly": {"G", "PG", "TV-G", "TV-Y", "TV-Y7", "all ages", "kid friendly"},
	"kids":            {"G", "TV-G", "TV-Y", "TV-Y7", "children", "animated kids"},
	"teen":            {"PG-13", "TV-14", "teenager", "young adult", "YA"},
	"mature":          {"R", "TV-MA", "adult", "mature themes", "graphic"},
	"adult only":      {"NC-17", "X", "unrated", "explicit", "adults only"},
	"all ages":        {"G", "PG", "TV-G", "TV-PG", "universal", "general audience"},
}

// LanguageExpansions maps language names to ISO codes and regional terms.
var LanguageExpansions = map[string]struct {
	ISOCode  string
	Variants []string
}{
	"english":      {"en", []string{"english", "american", "british", "australian", "hollywood"}},
	"korean":       {"ko", []string{"korean", "k-drama", "hallyu", "korean wave", "south korean"}},
	"japanese":     {"ja", []string{"japanese", "j-drama", "anime", "japan", "nihon"}},
	"chinese":      {"zh", []string{"chinese", "mandarin", "cantonese", "c-drama", "hong kong", "taiwanese"}},
	"spanish":      {"es", []string{"spanish", "latin", "telenovela", "spanish language", "hispanic"}},
	"french":       {"fr", []string{"french", "france", "francophone", "french cinema"}},
	"german":       {"de", []string{"german", "germany", "deutsch", "german cinema"}},
	"italian":      {"it", []string{"italian", "italy", "neorealism", "italian cinema"}},
	"hindi":        {"hi", []string{"hindi", "bollywood", "indian", "india", "desi"}},
	"thai":         {"th", []string{"thai", "thailand", "lakorn", "thai drama"}},
	"turkish":      {"tr", []string{"turkish", "turkey", "dizi", "turkish drama"}},
	"portuguese":   {"pt", []string{"portuguese", "brazilian", "brazil", "novela"}},
	"russian":      {"ru", []string{"russian", "russia", "soviet", "russian cinema"}},
	"scandinavian": {"", []string{"scandinavian", "nordic", "swedish", "norwegian", "danish", "finnish", "nordic noir"}},
}

// CountryExpansions maps country references to searchable terms.
var CountryExpansions = map[string][]string{
	"hollywood":      {"united states", "american", "usa", "hollywood", "us"},
	"bollywood":      {"india", "indian", "bollywood", "hindi", "mumbai"},
	"korean":         {"south korea", "korean", "seoul", "k-drama", "hallyu"},
	"british":        {"united kingdom", "british", "uk", "england", "bbc"},
	"french":         {"france", "french", "paris", "french cinema"},
	"japanese":       {"japan", "japanese", "tokyo", "anime"},
	"chinese":        {"china", "chinese", "hong kong", "taiwanese", "mandarin"},
	"european":       {"europe", "european", "eu", "continental"},
	"scandinavian":   {"sweden", "norway", "denmark", "finland", "nordic"},
	"latin":          {"latin america", "mexico", "argentina", "brazil", "spanish"},
	"african":        {"africa", "african", "nollywood", "nigerian"},
	"middle eastern": {"middle east", "arab", "iranian", "turkish", "israeli"},
}

// MoodExpansions maps mood descriptors to related emotional terms.
var MoodExpansions = map[string][]string{
	// Positive moods
	"uplifting": {"uplifting", "inspiring", "heartwarming", "feel-good", "hopeful", "optimistic"},
	"funny":     {"funny", "hilarious", "comedic", "witty", "humorous", "laugh out loud"},
	"romantic":  {"romantic", "love", "passion", "tender", "intimate", "heartfelt"},
	"exciting":  {"exciting", "thrilling", "exhilarating", "adrenaline", "action-packed"},
	"whimsical": {"whimsical", "quirky", "charming", "playful", "lighthearted", "delightful"},

	// Negative/intense moods
	"dark":       {"dark", "grim", "bleak", "gritty", "noir", "shadowy"},
	"scary":      {"scary", "frightening", "terrifying", "creepy", "chilling", "nightmare"},
	"tense":      {"tense", "suspenseful", "nail-biting", "edge of seat", "gripping"},
	"sad":        {"sad", "melancholy", "tragic", "tearjerker", "heartbreaking", "bittersweet"},
	"disturbing": {"disturbing", "unsettling", "uncomfortable", "provocative", "shocking"},

	// Atmospheric moods
	"atmospheric": {"atmospheric", "moody", "immersive", "evocative", "cinematic"},
	"dreamy":      {"dreamy", "surreal", "ethereal", "hypnotic", "otherworldly"},
	"nostalgic":   {"nostalgic", "sentimental", "reminiscent", "throwback", "retro"},
	"cozy":        {"cozy", "comforting", "warm", "hygge", "snug", "comfort viewing"},
	"epic":        {"epic", "grand", "sweeping", "majestic", "monumental", "spectacular"},

	// Pacing moods
	"slow burn":  {"slow burn", "deliberate", "meditative", "contemplative", "patient"},
	"fast paced": {"fast paced", "action-packed", "rapid", "breakneck", "non-stop"},

	// Intellectual moods
	"thought provoking": {"thought provoking", "intellectual", "cerebral", "philosophical", "deep"},
	"mind bending":      {"mind bending", "twisty", "complex", "puzzle", "non-linear"},
}

// ExpandGenre returns expanded search terms for a genre query.
func ExpandGenre(genre string) []string {
	normalized := strings.ToLower(strings.TrimSpace(genre))
	if terms, ok := GenreExpansions[normalized]; ok {
		return terms
	}
	return []string{genre}
}

// ExpandMood returns expanded search terms for a mood query.
func ExpandMood(mood string) []string {
	normalized := strings.ToLower(strings.TrimSpace(mood))
	if terms, ok := MoodExpansions[normalized]; ok {
		return terms
	}
	return []string{mood}
}

// GetEraYearRange returns the year range for an era descriptor.
func GetEraYearRange(era string) (startYear, endYear int, ok bool) {
	normalized := strings.ToLower(strings.TrimSpace(era))
	if info, exists := EraExpansions[normalized]; exists {
		return info.YearStart, info.YearEnd, true
	}
	return 0, 0, false
}

// GetLanguageCode returns the ISO code for a language name.
func GetLanguageCode(language string) (code string, ok bool) {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if info, exists := LanguageExpansions[normalized]; exists {
		return info.ISOCode, true
	}
	return "", false
}

// ExpandLanguage returns expanded search terms for a language query.
func ExpandLanguage(language string) []string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if info, ok := LanguageExpansions[normalized]; ok {
		return info.Variants
	}
	return []string{language}
}

// ExpandCountry returns expanded search terms for a country query.
func ExpandCountry(country string) []string {
	normalized := strings.ToLower(strings.TrimSpace(country))
	if terms, ok := CountryExpansions[normalized]; ok {
		return terms
	}
	return []string{country}
}
