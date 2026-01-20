# Homepage Redesign: The Conversational Home

## Design Philosophy

**Goal**: Transform the homepage from a "content warehouse" into a **conversation with a knowledgeable friend** who happens to know your entire library.

**Core Insight**: The best movie recommendations come from friends who ask "What are you in the mood for?" — not from walls of posters. ViewRA should feel like that friend.

**Key Differentiators**:

1. **Dialogue-driven** — the UI asks questions and responds, not just displays
2. **Opinionated curation** — fewer choices, presented with conviction
3. **Recommendation transparency** — always explain *why*
4. **Context awareness** — time, weather, mood shape the conversation
5. **Progressive disclosure** — start simple, reveal depth on demand

---

## The Conversational Flow

The homepage is structured as a **dialogue**, not a dashboard. Each section is a "turn" in the conversation.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│                         TURN 1: THE GREETING                                │
│                                                                             │
│  "Evening, Alex. Rainy Sunday — perfect for a movie."                      │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                      TURN 2: THE CONFIDENT PICK                             │
│                                                                             │
│  "I think you'd love this tonight:"                                        │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                                                                     │   │
│  │                        [Backdrop Image]                             │   │
│  │                                                                     │   │
│  │   GLASS ONION                                                       │   │
│  │   2022 · 2h 19m · Mystery                                          │   │
│  │                                                                     │   │
│  │   "You loved Knives Out — this is Rian Johnson's follow-up,        │   │
│  │    same sharp wit, new puzzle."                                     │   │
│  │                                                                     │   │
│  │   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐            │   │
│  │   │   ▶ Watch   │    │  Not tonight │    │  Tell me more│           │   │
│  │   └─────────────┘    └─────────────┘    └─────────────┘            │   │
│  │                                                                     │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                      TURN 3: THE CHECK-IN                                   │
│                                                                             │
│  "Or, pick up where you left off:"                                         │
│                                                                             │
│  ┌──────────────────────────────┐  ┌──────────────────────────────┐        │
│  │  [Backdrop]                  │  │  [Backdrop]                  │        │
│  │  ▶                           │  │  ▶                           │        │
│  │  ════════════░░░░░░          │  │  ═══════░░░░░░░░░░           │        │
│  │  Dune: Part Two              │  │  Shōgun · S1 E4              │        │
│  │  "47 minutes left"           │  │  "Eightfold Fence"           │        │
│  └──────────────────────────────┘  └──────────────────────────────┘        │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                      TURN 4: THE QUESTION                                   │
│                                                                             │
│  "Not feeling any of those? What sounds good?"                             │
│                                                                             │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐              │
│  │ Something│ │ Edge of │ │ Make me │ │ I have  │ │ Surprise│              │
│  │ cozy     │ │ my seat │ │ laugh   │ │ 90 mins │ │ me      │              │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘              │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Or tell me what you're looking for...                              │   │
│  │  "something like Parasite" · "80s horror" · "feel-good documentary" │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                      TURN 5: THE DEEP CUT                                   │
│                                                                             │
│  "By the way — today I'm spotlighting Christopher Nolan.                   │
│   Mind-bending stuff, if you're in that headspace."                        │
│                                                                             │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐     → │
│  │Inception│ │ Tenet  │ │Interst-│ │Memento │ │Prestige│ │  Dark  │       │
│  │        │ │        │ │ ellar  │ │        │ │        │ │ Knight │       │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                      TURN 6: THE BROWSE OPTION                              │
│                                                                             │
│  "Want to browse instead? Here's what's in your library:"                  │
│                                                                             │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐  │
│  │ Recently      │ │ Hidden Gems   │ │ Your          │ │ All Movies    │  │
│  │ Added         │ │ you've missed │ │ Favorites     │ │ & Shows       │  │
│  │               │ │               │ │               │ │               │  │
│  │ 12 new this   │ │ Highly rated, │ │ 47 titles     │ │ 1,247 titles  │  │
│  │ week          │ │ never watched │ │               │ │               │  │
│  └───────────────┘ └───────────────┘ └───────────────┘ └───────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## The Key Shift: Dialogue, Not Display

| Old Pattern                | New Pattern                                            |
| -------------------------- | ------------------------------------------------------ |
| "Continue Watching"        | "Pick up where you left off:"                          |
| "Recommended For You"      | "I think you'd love this tonight:"                     |
| "Categories"               | "What sounds good?"                                    |
| Mood chips                 | Intent phrases ("Edge of my seat", "Make me laugh")    |
| "See All"                  | "Want to browse instead?"                              |
| Silent poster grid         | Explanatory copy with every recommendation             |

**Every section has a voice.** The UI speaks to you.

---

## Component Specifications

### Turn 1: The Greeting

**Purpose**: Set the tone — we're having a conversation, not browsing a catalog.

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  "Evening, Alex. Rainy Sunday — perfect for a movie."      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Greeting Variants** (context-aware):

| Context | Example |
| ------- | ------- |
| Weekday evening | "Evening, Alex. Long day? Let's find something good." |
| Weekend morning | "Morning, Alex. Lazy Sunday — nowhere to be." |
| Late night | "Still up, Alex? I've got some late-night picks." |
| Rainy day | "Rainy afternoon, Alex. Perfect excuse to stay in." |
| Holiday season | "Happy holidays, Alex. In the mood for something festive?" |
| User has continue watching | "Welcome back, Alex. Ready to finish Dune?" |
| New user (cold start) | "Hey Alex — let's figure out what you like." |

**Behavior**:

- Single line of conversational text, not a header+subheader
- Combines time, weather (if available), and user context
- Feels written by a person, not generated
- Changes each visit (slight variation)

**Data Source**: `ContextEnricher.GetContext()` + `HeroData`

---

### Turn 2: The Confident Pick

**Purpose**: Lead with a single, strong recommendation. Not "here are some options" — "I think you'd love this."

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  "I think you'd love this tonight:"                                    │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                                                                   │ │
│  │                      [Backdrop Image]                             │ │
│  │                                                                   │ │
│  │                                                                   │ │
│  │   GLASS ONION                                                     │ │
│  │   2022 · 2h 19m · Mystery, Comedy                                │ │
│  │                                                                   │ │
│  │   "You loved Knives Out — this is Rian Johnson's follow-up,      │ │
│  │    same sharp wit, new puzzle."                                   │ │
│  │                                                                   │ │
│  │   ┌───────────┐   ┌─────────────┐   ┌───────────────┐            │ │
│  │   │  ▶ Watch  │   │ Not tonight │   │ Tell me more  │            │ │
│  │   └───────────┘   └─────────────┘   └───────────────┘            │ │
│  │                                                                   │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**The Key Difference**: A conversational blurb, not a label.

| Netflix/Plex Style | ViewRA Style |
| ------------------ | ------------ |
| "Because you watched Knives Out" | "You loved Knives Out — this is Rian Johnson's follow-up, same sharp wit, new puzzle." |
| "Top Pick for Alex" | "I think you'd love this tonight:" |
| "97% Match" | "Same director, same vibe, new mystery to solve." |

**Blurb Generation** (from backend or templates):

The blurb should feel written, not computed. Examples:

- **Sequel/Same director**: "You loved [X] — this is [director]'s follow-up, [quality description]."
- **Same genre/vibe**: "If you're in the mood for [mood] like [X], this one nails it."
- **Hidden gem**: "This flew under the radar, but it's right up your alley."
- **Highly rated**: "One of the best [genre] films of [decade]. You haven't seen it yet."
- **Seasonal**: "Perfect for a [rainy/cozy/late] night like this."

**Three Actions**:

| Button | Behavior |
| ------ | -------- |
| **▶ Watch** | Start playback immediately |
| **Not tonight** | Dismiss, show next pick (with smooth transition) |
| **Tell me more** | Expand to show full synopsis, cast, trailer |

**Data Sources**:

- `RecommendationsService.GetBecauseYouLiked()` → item + reason source
- `MoodTagService` → mood descriptors for blurb generation
- Custom blurb templates (frontend) based on reason type

---

### Turn 3: The Check-In

**Purpose**: Acknowledge what's in progress — but as a gentle aside, not the main event.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  "Or, pick up where you left off:"                                     │
│                                                                         │
│  ┌─────────────────────────────┐   ┌─────────────────────────────┐     │
│  │  [Backdrop]                 │   │  [Backdrop]                 │     │
│  │               ▶             │   │  S1 E4            ▶         │     │
│  │                             │   │                             │     │
│  │  ═══════════════░░░░░      │   │  ════════░░░░░░░░░░░░░░    │     │
│  │                             │   │                             │     │
│  │  Dune: Part Two             │   │  Shōgun                     │     │
│  │  "47 minutes left —         │   │  "Eightfold Fence"          │     │
│  │   the finale awaits"        │   │                             │     │
│  └─────────────────────────────┘   └─────────────────────────────┘     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Conversational Copy for Continue Cards**:

| Situation | Copy |
| --------- | ---- |
| Almost done (>80%) | "47 minutes left — the finale awaits" |
| Just started (<20%) | "You just started this one" |
| Mid-watch | "Right where you left off" |
| Been a while (>7 days) | "It's been a week — want to jump back in?" |
| TV show next episode | "Ready for episode 5?" |

**Empty State** (no continue watching):

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  "Nothing in progress — let's start something new."                    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

This section collapses/hides entirely if empty, making room for the pick above.

**Data Source**: `ContinueWatchingData` (existing)

---

### Turn 4: The Question

**Purpose**: Hand control to the user — "What are *you* in the mood for?"

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  "Not feeling any of those? What sounds good?"                         │
│                                                                         │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌─────────────┐ │
│  │  Something    │ │  Edge of      │ │  Make me      │ │  I've only  │ │
│  │  cozy         │ │  my seat      │ │  laugh        │ │  got an hour│ │
│  └───────────────┘ └───────────────┘ └───────────────┘ └─────────────┘ │
│                                                                         │
│  ┌───────────────┐ ┌───────────────┐                                   │
│  │  Surprise     │ │  Something    │                                   │
│  │  me           │ │  different    │                                   │
│  └───────────────┘ └───────────────┘                                   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Or tell me what you're looking for...                          │   │
│  │                                                                 │   │
│  │  "something like Parasite" · "90s action" · "make me cry"      │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Intent Options** (not mood labels — phrases a human would say):

| Intent Phrase | Backend Query | What It Finds |
| ------------- | ------------- | ------------- |
| "Something cozy" | mood:cozy + weather context | Comfort watches |
| "Edge of my seat" | mood:tense,thriller | Thrillers, suspense |
| "Make me laugh" | mood:fun,comedy | Comedies |
| "I've only got an hour" | runtime < 70min | Short films, episodes |
| "Surprise me" | random from top recommendations | Dealer's choice |
| "Something different" | exploration mode | Outside usual taste |

**Natural Language Input**:

The search field accepts conversational queries:

- "something like Parasite" → semantic similarity search
- "90s action movies" → genre + decade filter
- "make me cry" → mood search for emotional dramas
- "Christopher Nolan" → director search
- "I want to think" → thought-provoking recommendations

**Data Source**: Semantic search plugin + `MoodTagService`

---

### Turn 5: The Deep Cut

**Purpose**: Offer an editorial recommendation — "by the way, here's something interesting"

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  "By the way — today I'm spotlighting Christopher Nolan.               │
│   Mind-bending stuff, if you're in that headspace."                    │
│                                                                         │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐  → │
│  │Inception│ │ Tenet  │ │Interst-│ │Memento │ │Prestige│ │ Dark   │    │
│  │        │ │        │ │ ellar  │ │        │ │        │ │ Knight │    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘ └────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Spotlight Introduction Variants**:

| Type | Introduction |
| ---- | ------------ |
| Director | "Today I'm spotlighting [Director]. [Description]." |
| Hidden gems | "These flew under the radar, but they're gems." |
| Classic cinema | "Feeling nostalgic? Here's some golden age cinema." |
| Holiday | "Getting into the holiday spirit? I've got you." |
| Genre deep-dive | "In the mood for [genre]? Here's the best of it." |

**Key Difference from Standard Row**:

- Has a **conversational introduction**, not just a title
- Feels like a recommendation from a person
- Changes daily (rotation from `ThemedRecommendations`)

**Data Source**: `ThemedRecommendations.getCuratedRows()` + `GetActiveThemedRows()`

---

### Turn 6: The Browse Fallback

**Purpose**: For users who want to explore on their own — "here's the library if you want it"

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  "Want to browse instead? Here's your library:"                        │
│                                                                         │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐           │
│  │                 │ │                 │ │                 │           │
│  │  Recently       │ │  Hidden Gems    │ │  All Movies     │           │
│  │  Added          │ │                 │ │  & Shows        │           │
│  │                 │ │  "Highly rated  │ │                 │           │
│  │  "12 new this   │ │   but you       │ │  "1,247 titles" │           │
│  │   week"         │ │   haven't seen" │ │                 │           │
│  │                 │ │                 │ │                 │           │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Browse Cards** (not poster grids — entry points):

| Card | Description | Destination |
| ---- | ----------- | ----------- |
| Recently Added | "12 new this week" | `/browse/recent` |
| Hidden Gems | "Highly rated but you haven't seen" | `/browse/hidden-gems` |
| Your Favorites | "47 titles you've loved" | `/browse/favorites` |
| All Movies & Shows | "1,247 titles" | `/browse` |

**Why Cards, Not Rows**:

- Rows encourage endless scrolling
- Cards are **entry points** to focused browse experiences
- Keeps the homepage as a conversation, not a catalog

**Data Source**: Existing section data + library stats

---

## Responsive Behavior

The conversational flow adapts to screen size while maintaining the dialogue structure.

### Desktop (1200px+)

```text
┌─────────────────────────────────────────────────────────────────┐
│  "Evening, Alex. Rainy Sunday — perfect for a movie."          │
├─────────────────────────────────────────────────────────────────┤
│  "I think you'd love this tonight:"                            │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  [Featured Pick — full width hero card]                   │ │
│  └───────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  "Or, pick up where you left off:"                             │
│  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │  [Continue 1]       │  │  [Continue 2]       │              │
│  └─────────────────────┘  └─────────────────────┘              │
├─────────────────────────────────────────────────────────────────┤
│  "Not feeling any of those? What sounds good?"                 │
│  [Intent chips]  [Natural language input]                      │
├─────────────────────────────────────────────────────────────────┤
│  "By the way — today I'm spotlighting..."                      │
│  [Spotlight row]                                               │
├─────────────────────────────────────────────────────────────────┤
│  "Want to browse instead?"                                     │
│  [Browse cards]                                                │
└─────────────────────────────────────────────────────────────────┘
```

### Mobile (< 768px)

```text
┌──────────────────────────┐
│  "Evening, Alex..."      │
├──────────────────────────┤
│  "I think you'd love     │
│   this tonight:"         │
│  ┌────────────────────┐  │
│  │  [Featured Pick]   │  │
│  │  [Full width]      │  │
│  └────────────────────┘  │
├──────────────────────────┤
│  "Pick up where you      │
│   left off:"             │
│  [Horizontal scroll →]   │
├──────────────────────────┤
│  "What sounds good?"     │
│  [Chips scroll →]        │
│  [Search input]          │
├──────────────────────────┤
│  "By the way..."         │
│  [Spotlight scroll →]    │
├──────────────────────────┤
│  "Want to browse?"       │
│  [Browse cards stack]    │
└──────────────────────────┘
```

**Key Mobile Adaptations**:

- Conversational prompts stay, but shorter
- Cards go full-width or horizontal scroll
- Intent chips become scrollable
- Vertical stacking maintains reading order

---

## Data Requirements

### New API Response Structure

The `/api/home` endpoint returns a conversational structure:

```typescript
interface ConversationalHomeResponse {
  // Turn 1: The Greeting
  greeting: {
    text: string              // "Evening, Alex. Rainy Sunday — perfect for a movie."
    context: {
      time_of_day: 'morning' | 'afternoon' | 'evening' | 'night'
      day_of_week: string
      weather?: {
        condition: string     // "rainy", "sunny", etc.
        temperature?: number
      }
      has_continue_watching: boolean
    }
  }

  // Turn 2: The Confident Pick
  featured_pick: {
    media: MediaItem
    intro: string             // "I think you'd love this tonight:"
    blurb: string             // "You loved Knives Out — this is Rian Johnson's follow-up..."
    blurb_type: 'sequel' | 'same_director' | 'similar_vibe' | 'hidden_gem' | 'seasonal'
    source?: {                // What the recommendation is based on
      media_id: number
      title: string
    }
  }

  // Turn 3: The Check-In
  continue_watching: {
    intro: string             // "Or, pick up where you left off:"
    items: ContinueItem[]
    empty_message?: string    // "Nothing in progress — let's start something new."
  }

  // Turn 4: The Question
  mood_selector: {
    intro: string             // "Not feeling any of those? What sounds good?"
    intents: IntentOption[]
    search_placeholder: string
    search_examples: string[] // ["something like Parasite", "90s action"]
  }

  // Turn 5: The Deep Cut
  spotlight: {
    intro: string             // "By the way — today I'm spotlighting Christopher Nolan."
    title: string             // "Christopher Nolan"
    description: string       // "Mind-bending stuff, if you're in that headspace."
    type: 'director' | 'hidden_gems' | 'classic' | 'holiday' | 'genre'
    items: MediaItem[]
  }

  // Turn 6: The Browse Fallback
  browse_options: {
    intro: string             // "Want to browse instead? Here's your library:"
    cards: BrowseCard[]
  }
}

interface IntentOption {
  id: string
  phrase: string              // "Something cozy" (not "Cozy")
  query: string               // Backend search query
}

interface ContinueItem extends MediaItem {
  progress_percent: number
  time_remaining: string      // "47 minutes"
  contextual_copy: string     // "the finale awaits" or "right where you left off"
}

interface BrowseCard {
  id: string
  title: string               // "Recently Added"
  description: string         // "12 new this week"
  url: string
}
```

### Backend Changes

1. **New greeting generator** — context-aware conversational greetings
2. **Blurb generator for featured pick** — templates based on recommendation reason
3. **Contextual copy for continue watching** — based on progress, time since last watch
4. **Intent-to-query mapping** — natural language phrases → search queries
5. **Spotlight intro generator** — conversational intros for curated rows

---

## Component Hierarchy

```text
ConversationalHome/
├── Turn1_Greeting/
│   └── GreetingText (animated typewriter effect optional)
├── Turn2_ConfidentPick/
│   ├── IntroText
│   ├── FeaturedCard/
│   │   ├── BackdropImage
│   │   ├── TitleBlock
│   │   ├── ConversationalBlurb
│   │   └── ActionButtons (Watch, Not tonight, Tell me more)
│   └── DismissedState (shows next pick)
├── Turn3_CheckIn/
│   ├── IntroText
│   ├── ContinueCards/
│   │   └── ContinueCard (with contextual copy)
│   └── EmptyState
├── Turn4_Question/
│   ├── IntroText
│   ├── IntentChips/
│   │   └── IntentChip (phrase-based, not label-based)
│   └── NaturalLanguageInput
├── Turn5_DeepCut/
│   ├── IntroText (conversational)
│   └── SpotlightRow
└── Turn6_BrowseFallback/
    ├── IntroText
    └── BrowseCards
```

---

## Interaction Details

### "Not tonight" Flow

When user dismisses the featured pick:

1. Card slides out left (200ms)
2. New pick slides in from right (200ms)
3. Blurb updates: "Okay, how about this instead:"
4. After 3 dismissals: "Hmm, picky tonight. Tell me what you're looking for." → focus search input

### Natural Language Input

The search field is **conversational**, not a traditional search box:

- Placeholder cycles: "something like Parasite" → "90s action" → "make me cry"
- On focus: Show recent searches + suggested queries
- On submit: Semantic search with conversational results page

### Page Load Animation

Staggered reveal that feels like the UI is "speaking":

1. Greeting types in (0-400ms) — optional typewriter effect
2. Featured pick fades up (400ms)
3. Continue section fades in (500ms)
4. Question section fades in (600ms)
5. Spotlight fades in (700ms)
6. Browse cards fade in (800ms)

All animations use `ease-out` curve for natural feel.

---

## Visual Design Notes

### Typography for Conversation

The conversational text needs to feel **written**, not **UI copy**:

- Use a slightly larger font for intro text (18-20px)
- Softer weight (400-500, not bold headers)
- Generous line height (1.5+)
- Quotation style optional — the informal tone signals conversation

### Color Palette

| Element                  | Light Mode       | Dark Mode         |
| ------------------------ | ---------------- | ----------------- |
| Background               | neutral-50       | neutral-950       |
| Featured card bg         | white            | neutral-900       |
| Conversational text      | neutral-700      | neutral-300       |
| Intent chips             | neutral-100      | neutral-800       |
| Intent chips (hover)     | primary-100      | primary-900       |
| Text primary             | neutral-900      | neutral-50        |
| Text secondary           | neutral-600      | neutral-400       |

### Tone of Voice

The UI should sound like:

- A friend who knows movies well
- Casual but not try-hard
- Confident recommendations, not hedging
- Brief — never verbose

**Do say**: "I think you'd love this tonight"
**Don't say**: "Based on your viewing history, we recommend"

**Do say**: "Not tonight" (button)
**Don't say**: "Skip" or "Dismiss"

**Do say**: "Hmm, picky tonight. Tell me what you're looking for."
**Don't say**: "No recommendations match your preferences."

---

## Success Metrics

1. **Time to play** — How quickly do users start watching?
2. **Featured pick acceptance rate** — Do users click the confident pick?
3. **"Not tonight" usage** — Are we missing the mark?
4. **Intent chip clicks** — Which moods resonate?
5. **Natural language search usage** — Are users conversing?
6. **Scroll depth** — Do users need to browse, or is the conversation enough?

---

## Implementation Phases

### Phase 1: Conversational Foundation

- Greeting generator with context awareness
- Featured pick with conversational blurb
- "Not tonight" dismiss flow
- Continue watching with contextual copy

### Phase 2: The Question

- Intent chips (phrase-based)
- Natural language input
- Semantic search integration

### Phase 3: Deep Cut & Browse

- Spotlight section with intro text
- Browse fallback cards
- Animation polish

### Phase 4: Full Context

- Weather integration (opt-in)
- Holiday theming
- Blurb generation refinements
- A/B testing different tones
