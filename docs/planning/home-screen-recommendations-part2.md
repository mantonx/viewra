# Home Screen Enhancement Plan - Part 2

## Research & Industry Analysis

This document captures research findings from analyzing open-source media applications (Jellyfin, Streamyfin, Blink, Netflix UI clones) and defines the enhanced implementation plan for ViewRA's home screen.

---

## Industry Patterns & Best Practices

### 1. Continue Watching Card Design

**Key Finding:** Premium media apps use **horizontal 16:9 thumbnails** for continue watching, not vertical posters.

**Reference Implementation (Streamyfin):**

```tsx
// components/ContinueWatchingPoster.tsx
<View className="relative w-44 aspect-video rounded-lg overflow-hidden border border-neutral-800">
  <Image source={{ uri: url }} contentFit="cover" className="w-full h-full" />
  {showPlayButton && (
    <View className="absolute inset-0 flex items-center justify-center">
      <Ionicons name="play-circle" size={40} color="white" />
    </View>
  )}
  {!item.UserData?.Played && <WatchedIndicator item={item} />}
  <ProgressBar item={item} />
</View>
```

**Progress Bar (Streamyfin):**

```tsx
// components/common/ProgressBar.tsx
export const ProgressBar = ({ item }) => {
  const progress = item.UserData?.PlayedPercentage || 0;
  if (progress <= 0) return null;

  return (
    <>
      <View className="absolute bottom-0 left-0 h-1 bg-neutral-700 opacity-80 w-full" />
      <View
        style={{ width: `${progress}%` }}
        className="absolute bottom-0 left-0 h-1 bg-purple-600 w-full"
      />
    </>
  );
};
```

**Why horizontal works better:**
- Shows actual scene from where user left off
- Progress bar is more visible on wider cards
- Episode thumbnails are naturally 16:9
- Distinguishes "in progress" from "browse" rows

### 2. Home Section Structure

**Jellyfin Vue Pattern:**

```typescript
const homeSections = [
  { type: 'libraries', title: 'Libraries' },           // Library tiles
  { type: 'resumevideo', title: 'Continue Watching' }, // In-progress items
  { type: 'nextup', title: 'Next Up' },                // Next episode for TV
  ...latestMediaSections                                // Latest per library
];
```

**Key Insight:** "Next Up" is distinct from "Continue Watching"

| Row | Content | Use Case |
|-----|---------|----------|
| Continue Watching | Movies/episodes with progress > 0% and < 90% | Resume what you started |
| Next Up | Next unwatched episode in series you've started | Auto-play next episode |
| Recently Added | New content in library | Discovery |

### 3. Hero/Featured Content

**Netflix UI Clone Pattern:**

```tsx
<View style={styles.featuredContent}>
  {/* Background image */}
  <Animated.Image source={{ uri: movie.thumbnail }} style={imageStyle} />
  
  {/* Gradient fade for text readability */}
  <LinearGradient
    colors={['transparent', 'rgba(0,0,0,0.8)']}
    style={styles.featuredGradient}
  />
  
  {/* Movie logo (if available) instead of text */}
  <Animated.Image source={{ uri: movie.logo }} style={styles.featuredLogo} />
  
  {/* Categories/genres */}
  <Text style={styles.categoriesText}>
    {movie.categories.join(' • ')}
  </Text>
  
  {/* Primary actions */}
  <View style={styles.featuredButtons}>
    <Pressable style={styles.playButton}>
      <Ionicons name="play" size={24} color="#000" />
      <Text>Play</Text>
    </Pressable>
    <Pressable style={styles.myListButton}>
      <Ionicons name="add" size={24} color="#fff" />
      <Text>My List</Text>
    </Pressable>
  </View>
</View>
```

### 4. Backdrop with Parallax (Blink)

```tsx
// components/itemBackdrop/index.tsx
function ItemBackdropContent({ targetRef, src, alt, distance }) {
  const { scrollYProgress } = useScroll({
    target: targetRef,
    offset: ["start start", "60vh start"],
  });
  
  const y = useTransform(scrollYProgress, [0, 1], [-distance, distance]);
  
  return (
    <motion.img
      src={src}
      className="item-hero-backdrop"
      style={{ y }}
      onLoad={(e) => { e.currentTarget.style.opacity = "1"; }}
    />
  );
}
```

### 5. Time Remaining Formatting

**Common Pattern:**

```typescript
function formatRemainingTime(remainingSeconds: number): string {
  const hours = Math.floor(remainingSeconds / 3600);
  const mins = Math.floor((remainingSeconds % 3600) / 60);
  
  if (hours > 0) {
    return `${hours}h ${mins}m left`;
  }
  return `${mins} min left`;
}

// Calculate remaining from progress
function getRemainingTime(positionSeconds: number, durationSeconds: number): string {
  const remaining = durationSeconds - positionSeconds;
  return formatRemainingTime(remaining);
}
```

### 6. Card Shapes by Content Type

| Content Type | Aspect Ratio | CSS Class | Reason |
|--------------|--------------|-----------|--------|
| Movie Poster | 2:3 | `aspect-[2/3]` | Standard poster ratio |
| Continue Watching | 16:9 | `aspect-video` | Shows scene, progress visible |
| TV Episode | 16:9 | `aspect-video` | Episode thumbnails |
| Music Album | 1:1 | `aspect-square` | Album art standard |
| TV Season | 2:3 | `aspect-[2/3]` | Season poster |

### 7. Progress Thresholds (Jellyfin Standard)

```typescript
const MINIMUM_PERCENTAGE = 5;    // 5% minimum to save progress
const PLAYED_THRESHOLD = 90;     // 90%+ marks as "played"

// Item states:
// - 0%: Not started
// - 5-90%: In progress (show in Continue Watching)
// - 90%+: Played (show checkmark, exclude from Continue Watching)
```

### 8. BlurHash for Loading States

Jellyfin returns `ImageBlurHashes` with each item for smooth loading:

```typescript
// API response includes blur hashes
interface BaseItemDto {
  ImageBlurHashes?: {
    Primary?: Record<string, string>;
    Backdrop?: Record<string, string>;
    Thumb?: Record<string, string>;
  };
}

// Usage
const blurhash = item.ImageBlurHashes?.Primary?.[item.ImageTags?.Primary];
```

**Benefits:**
- Show blurred placeholder while image loads
- Smooth transition from blur to clear
- Better perceived performance

---

## Gap Analysis: Current vs. Ideal

### Critical Gaps

| Gap | Current State | Ideal State | Priority |
|-----|---------------|-------------|----------|
| Continue Watching aspect ratio | 2:3 poster | 16:9 horizontal | High |
| Progress in Continue Watching | Not displayed | Progress bar + time remaining | High |
| Hero backdrop | Static glow effect | Dynamic backdrop from content | High |
| Time remaining | Not shown | "1h 23m left" text | High |

### Medium Priority Gaps

| Gap | Current State | Ideal State | Priority |
|-----|---------------|-------------|----------|
| Next Up row | Not implemented | Separate row for TV next episodes | Medium |
| Episode info on cards | Not shown | "S2 E4" badge | Medium |
| New badge | Not implemented | "New" pill on recent items | Medium |
| Row item counts | Not shown | "Recently Added (12)" | Medium |

### Nice to Have

| Gap | Current State | Ideal State | Priority |
|-----|---------------|-------------|----------|
| BlurHash loading | No placeholder | Blur-to-clear transition | Low |
| Movie logos | Text only | Logo image when available | Low |
| Parallax scroll | No effect | Subtle parallax on hero | Low |
| Ambient glow | Fixed glow | Dynamic color from backdrop | Low |

---

## Design Decisions

### Decision 1: Continue Watching Card Style

**Options:**
- A) Horizontal 16:9 cards (industry standard)
- B) Vertical 2:3 posters with overlay (consistent with other rows)
- C) Larger horizontal cards with more info

**Recommendation:** Option A - Horizontal 16:9

**Rationale:**
- Matches user expectations from Netflix, Jellyfin, Plex
- Progress bar more visible
- Shows scene context (where user left off)
- Episode thumbnails are naturally 16:9

### Decision 2: Next Up Row

**Options:**
- A) Add separate "Next Up" row for TV shows
- B) Include next episodes in Continue Watching
- C) Skip for now

**Recommendation:** Option B for MVP, Option A later

**Rationale:**
- Option B is simpler and still useful
- Continue Watching can show "S2 E4" badge for TV
- Add dedicated Next Up row in future iteration

### Decision 3: Hero Content Source

**Options:**
- A) First Continue Watching item
- B) First trending item
- C) Random featured item
- D) Most recently watched

**Recommendation:** Option A with fallback to B

**Rationale:**
- Continue Watching is most relevant to user
- Trending as fallback for new users
- Creates personal connection

### Decision 4: Time Remaining Format

**Options:**
- A) "1h 23m left"
- B) "1:23:00 remaining"
- C) "45 min left"
- D) Progress percentage only

**Recommendation:** Option A/C (adaptive)

**Rationale:**
- More readable than timestamps
- Matches Netflix, YouTube patterns
- Use "Xh Ym" for > 1 hour, "X min" for < 1 hour

---

## Enhanced Implementation Plan

### Phase 1: Continue Watching (Critical Fix)

#### Backend Changes

**1.1 Add progress data to continue watching response**

File: `internal/application/home/continue_watching.go`

```go
// MediaItemWithProgress extends MediaItemWithTime with progress info
type MediaItemWithProgress struct {
    MediaItemWithTime
    Progress *ProgressInfo `json:"progress,omitempty"`
}

type ProgressInfo struct {
    Percent         int    `json:"percent"`
    PositionSeconds int    `json:"position_seconds"`
    DurationSeconds int    `json:"duration_seconds"`
    RemainingText   string `json:"remaining_text"` // "1h 23m left"
    LastWatchedAt   string `json:"last_watched_at"`
}

func (s *ContinueWatchingServiceImpl) GetContinueWatchingWithProgress(
    ctx context.Context, userID string, limit int,
) ([]MediaItemWithProgress, error) {
    // 1. Get in-progress items from progress repo
    // 2. For each item, include the actual progress data
    // 3. Calculate remaining time text
    // 4. Return enriched items
}
```

**1.2 Add helper for formatting remaining time**

File: `internal/application/home/format.go`

```go
func FormatRemainingTime(positionSeconds, durationSeconds int) string {
    remaining := durationSeconds - positionSeconds
    if remaining <= 0 {
        return ""
    }
    
    hours := remaining / 3600
    mins := (remaining % 3600) / 60
    
    if hours > 0 {
        return fmt.Sprintf("%dh %dm left", hours, mins)
    }
    return fmt.Sprintf("%d min left", mins)
}
```

#### Frontend Changes

**1.3 Create ContinueWatchingCard component**

File: `web/src/components/home/widgets/ContinueWatchingCard.tsx`

```tsx
interface ContinueWatchingCardProps {
  mediaId: number;
  mediaType: 'movie' | 'tv-show';
  title: string;
  progress: {
    percent: number;
    remaining_text: string;
  };
  episodeInfo?: {
    season: number;
    episode: number;
  };
  onClick: () => void;
}

export const ContinueWatchingCard = ({
  mediaId,
  mediaType,
  title,
  progress,
  episodeInfo,
  onClick,
}: ContinueWatchingCardProps) => {
  return (
    <div className="w-56 shrink-0" onClick={onClick}>
      <div className="relative aspect-video rounded-xl overflow-hidden">
        {/* Thumbnail - use backdrop/thumb instead of poster */}
        <MediaBackdrop mediaId={mediaId} mediaType={mediaType} />
        
        {/* Play button overlay */}
        <HoverPlayButton />
        
        {/* Episode badge for TV */}
        {episodeInfo && (
          <div className="absolute top-2 left-2 px-2 py-1 bg-black/70 rounded text-xs text-white">
            S{episodeInfo.season} E{episodeInfo.episode}
          </div>
        )}
        
        {/* Progress bar */}
        <div className="absolute bottom-0 left-0 right-0 h-1 bg-neutral-700">
          <div 
            className="h-full bg-primary-500" 
            style={{ width: `${progress.percent}%` }} 
          />
        </div>
      </div>
      
      {/* Info */}
      <div className="mt-2">
        <h3 className="font-medium text-sm line-clamp-1">{title}</h3>
        <p className="text-xs text-neutral-500">{progress.remaining_text}</p>
      </div>
    </div>
  );
};
```

**1.4 Update ContinueRow to use new card**

File: `web/src/components/home/widgets/ContinueRow.tsx`

```tsx
export const ContinueRow = ({ data, className }: ContinueRowProps) => {
  const navigate = useNavigate();
  
  // Use horizontal cards for continue watching
  return (
    <section className={className}>
      <div className="flex items-center justify-between mb-5">
        <h2 className="text-xl font-display">Continue Watching</h2>
      </div>
      
      <ScrollableRow gap={4}>
        {data.items?.map((item) => (
          <ContinueWatchingCard
            key={item.entity_id}
            mediaId={item.entity_id}
            mediaType={item.entity_type}
            title={item.title}
            progress={item.progress}
            episodeInfo={item.episode_info}
            onClick={() => handleClick(item)}
          />
        ))}
      </ScrollableRow>
    </section>
  );
};
```

---

### Phase 2: Hero Section Redesign

#### Backend Changes

**2.1 Add hero data to home response**

File: `internal/domain/home/types.go`

```go
type HeroData struct {
    // BackdropMediaID is the media ID to fetch backdrop from
    BackdropMediaID int64  `json:"backdrop_media_id,omitempty"`
    BackdropType    string `json:"backdrop_type,omitempty"` // "movie" or "tv_show"
    
    // Greeting based on time of day
    Greeting string `json:"greeting,omitempty"` // "Good evening"
    
    // Current date formatted
    DateText string `json:"date_text,omitempty"` // "Saturday, January 3"
}
```

**2.2 Build hero data in service**

File: `internal/application/home/service.go`

```go
func (s *Service) buildHeroData(ctx context.Context, userID string) *home.HeroData {
    hero := &home.HeroData{
        Greeting: getGreeting(),
        DateText: time.Now().Format("Monday, January 2"),
    }
    
    // Try to get backdrop from continue watching
    if s.continueWatching != nil {
        items, err := s.continueWatching.GetContinueWatchingFull(ctx, userID, 1)
        if err == nil && len(items) > 0 {
            hero.BackdropMediaID = items[0].GetMediaID()
            hero.BackdropType = items[0].Type
            return hero
        }
    }
    
    // Fallback to trending
    if s.trending != nil && s.trending.HasProvider() {
        result, err := s.trending.GetTrending(ctx, "all", 1)
        if err == nil && len(result.Items) > 0 && result.Items[0].LocalID != nil {
            hero.BackdropMediaID = *result.Items[0].LocalID
            hero.BackdropType = result.Items[0].MediaType
        }
    }
    
    return hero
}

func getGreeting() string {
    hour := time.Now().Hour()
    switch {
    case hour >= 5 && hour < 12:
        return "Good morning"
    case hour >= 12 && hour < 17:
        return "Good afternoon"
    case hour >= 17 && hour < 21:
        return "Good evening"
    default:
        return "Good night"
    }
}
```

#### Frontend Changes

**2.3 Create HeroBackdrop component**

File: `web/src/components/home/HeroBackdrop.tsx`

```tsx
interface HeroBackdropProps {
  mediaId: number;
  mediaType: 'movie' | 'tv-show';
}

export const HeroBackdrop = ({ mediaId, mediaType }: HeroBackdropProps) => {
  const { images, isLoading } = useMediaImages(mediaId, mediaType);
  const backdropImage = images ? getFanartImage(images) : null;
  const backdropUrl = backdropImage ? getImageUrl(backdropImage.id, 'xlarge') : null;
  
  if (isLoading || !backdropUrl) {
    return null;
  }
  
  return (
    <div className="absolute inset-0 -z-10 overflow-hidden">
      {/* Backdrop image - blurred and desaturated */}
      <img
        src={backdropUrl}
        alt=""
        className="absolute inset-0 w-full h-full object-cover scale-110 blur-sm saturate-50 opacity-30"
      />
      
      {/* Gradient overlay */}
      <div className="absolute inset-0 bg-gradient-to-b from-transparent via-transparent to-[rgb(var(--color-bg-primary))]" />
      
      {/* Side gradients */}
      <div className="absolute inset-0 bg-gradient-to-r from-[rgb(var(--color-bg-primary))] via-transparent to-[rgb(var(--color-bg-primary))] opacity-50" />
    </div>
  );
};
```

**2.4 Update Home.tsx**

```tsx
export const Home = () => {
  const { data: homeSections } = useHomeSections();
  const heroData = homeSections?.meta?.hero;
  
  return (
    <div className="relative h-full overflow-auto">
      {/* Hero backdrop */}
      {heroData?.backdrop_media_id && (
        <HeroBackdrop
          mediaId={heroData.backdrop_media_id}
          mediaType={heroData.backdrop_type}
        />
      )}
      
      {/* Content */}
      <div className="relative z-10">
        {/* Greeting */}
        {heroData && (
          <div className="px-8 pt-8">
            <p className="text-sm text-neutral-500">{heroData.date_text}</p>
            <h1 className="text-2xl font-display">{heroData.greeting}</h1>
          </div>
        )}
        
        {/* Search Hero */}
        <div className="p-8 pb-4">
          <SearchHero data={searchHeroData} />
        </div>
        
        {/* Sections */}
        <div className="p-8 pt-4 space-y-10">
          <WidgetSection sections={sections} location="homepage-sections" />
        </div>
      </div>
    </div>
  );
};
```

---

### Phase 3: Row Polish

**3.1 NewBadge component**

```tsx
// web/src/components/media/NewBadge/NewBadge.tsx
interface NewBadgeProps {
  createdAt: string; // ISO date string
  daysThreshold?: number;
}

export const NewBadge = ({ createdAt, daysThreshold = 7 }: NewBadgeProps) => {
  const isNew = useMemo(() => {
    const created = new Date(createdAt);
    const now = new Date();
    const diffDays = (now.getTime() - created.getTime()) / (1000 * 60 * 60 * 24);
    return diffDays <= daysThreshold;
  }, [createdAt, daysThreshold]);
  
  if (!isNew) return null;
  
  return (
    <span className="absolute top-2 left-2 px-2 py-0.5 text-xs font-medium bg-primary-500 text-white rounded-full">
      New
    </span>
  );
};
```

**3.2 Row header with count**

```tsx
// Update MediaRow.tsx header
<div className="flex items-center justify-between mb-5">
  <div className="flex items-center gap-2">
    <h2 className="text-xl font-display">{data.title}</h2>
    {itemCount > 0 && (
      <span className="text-sm text-neutral-500">({itemCount})</span>
    )}
  </div>
  {seeAllUrl && <SeeAllLink to={seeAllUrl} />}
</div>
```

**3.3 MarkWatched button on hover**

```tsx
// web/src/components/media/MarkWatchedButton/MarkWatchedButton.tsx
export const MarkWatchedButton = ({ mediaId, isWatched, onToggle }) => {
  return (
    <button
      onClick={(e) => {
        e.stopPropagation();
        onToggle();
      }}
      className={cn(
        'absolute bottom-2 right-2 p-1.5 rounded-full transition-all',
        'opacity-0 group-hover:opacity-100',
        isWatched
          ? 'bg-green-500 text-white'
          : 'bg-black/50 text-white hover:bg-black/70'
      )}
      title={isWatched ? 'Mark as unwatched' : 'Mark as watched'}
    >
      <Check className="w-4 h-4" />
    </button>
  );
};
```

---

### Phase 4: Empty & Loading States

**4.1 Staggered skeleton animation**

```tsx
// HomeLoadingSkeleton with staggered animation
{[1, 2, 3, 4, 5, 6].map((card, index) => (
  <div
    key={card}
    className="w-48 shrink-0 skeleton-shimmer"
    style={{ animationDelay: `${index * 100}ms` }}
  >
    <div className="aspect-[2/3] bg-neutral-200 dark:bg-neutral-800 rounded-xl" />
  </div>
))}
```

**4.2 Contextual empty states**

```typescript
const emptyStateMessages: Record<string, { title: string; subtitle: string }> = {
  'continue-watching': {
    title: 'Nothing in progress',
    subtitle: 'Start watching something to see it here',
  },
  'favorites': {
    title: 'No favorites yet',
    subtitle: 'Heart items to add them to your favorites',
  },
  'recently-added': {
    title: 'No recent additions',
    subtitle: 'New content will appear here when added to your library',
  },
};
```

---

### Phase 5: Distinguishing Features

**5.1 Ambient glow from backdrop**

```tsx
// Extract dominant color using canvas
const extractDominantColor = async (imageUrl: string): Promise<string> => {
  const img = new Image();
  img.crossOrigin = 'anonymous';
  img.src = imageUrl;
  
  return new Promise((resolve) => {
    img.onload = () => {
      const canvas = document.createElement('canvas');
      const ctx = canvas.getContext('2d');
      canvas.width = 1;
      canvas.height = 1;
      ctx.drawImage(img, 0, 0, 1, 1);
      const [r, g, b] = ctx.getImageData(0, 0, 1, 1).data;
      resolve(`rgb(${r}, ${g}, ${b})`);
    };
    img.onerror = () => resolve('rgb(59, 130, 246)'); // Fallback to primary
  });
};
```

**5.2 Time-based row ordering**

Already partially implemented in `applySmartDefaults()`. Enhance:

```go
func (s *Service) applySmartDefaults(ctx context.Context, userID string, widgets []*registry.RegisteredWidget) []*registry.RegisteredWidget {
    timeOfDay := getTimeOfDay()
    
    for _, w := range result {
        switch w.Widget.ID {
        case "continue-watching":
            if timeOfDay == "morning" {
                // Boost in morning - finish what you started
                priorities[w.Widget.ID] += 60
            }
        case "trending":
            if timeOfDay == "evening" || timeOfDay == "night" {
                // Boost in evening - discovery time
                priorities[w.Widget.ID] += 20
            }
        }
    }
}
```

**5.3 Episode info badge for TV**

Update backend to include current episode info for TV shows in continue watching:

```go
type EpisodeInfo struct {
    Season  int    `json:"season"`
    Episode int    `json:"episode"`
    Title   string `json:"title,omitempty"`
}
```

**5.4 Keyboard navigation**

```tsx
// ScrollableRow with keyboard support
const handleKeyDown = (e: React.KeyboardEvent) => {
  const container = scrollRef.current;
  if (!container) return;
  
  switch (e.key) {
    case 'ArrowLeft':
      // Focus previous card
      break;
    case 'ArrowRight':
      // Focus next card
      break;
    case 'Enter':
      // Trigger click on focused card
      break;
  }
};
```

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/application/home/format.go` | Time formatting helpers |
| `web/src/components/home/HeroBackdrop.tsx` | Backdrop with blur/gradient |
| `web/src/components/home/widgets/ContinueWatchingCard.tsx` | Horizontal card with progress |
| `web/src/components/media/NewBadge/NewBadge.tsx` | "New" indicator |
| `web/src/components/media/MarkWatchedButton/MarkWatchedButton.tsx` | Quick action |
| `web/src/components/media/MediaBackdrop/MediaBackdrop.tsx` | 16:9 backdrop image |

### Modified Files

| File | Changes |
|------|---------|
| `internal/application/home/continue_watching.go` | Add progress data |
| `internal/application/home/service.go` | Add hero data, enhance smart defaults |
| `internal/domain/home/types.go` | Add HeroData, ProgressInfo types |
| `web/src/components/home/widgets/ContinueRow.tsx` | Use horizontal cards |
| `web/src/components/home/widgets/MediaRow.tsx` | Add counts, new badges |
| `web/src/views/home/Home.tsx` | Add hero backdrop, greeting |

---

## Open Questions

1. **Horizontal vs Vertical for Continue Watching** - Recommend horizontal, but needs design review

2. **Next Up Row** - Defer to future iteration or include now?

3. **BlurHash** - Requires significant backend work. Worth it for v1?

4. **Movie logos** - Only show if available, fallback to text title

---

## Implementation Order

1. **Phase 1: Continue Watching** (Critical) - 1 day
2. **Phase 2: Hero Backdrop** (High Impact) - 0.5 day
3. **Phase 3: Row Polish** (Medium) - 0.5 day
4. **Phase 4: Empty States** (Low) - 0.25 day
5. **Phase 5: Distinguishing Features** (Nice to Have) - 1 day

**Total estimated time: 3-4 days**

---

## Implementation Progress

### Status Summary

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1: Continue Watching | COMPLETE | Horizontal 16:9 cards with progress bars |
| Phase 2: Hero Backdrop | COMPLETE | Dynamic backdrop from continue watching |
| Phase 3: Row Polish | PENDING | NewBadge, counts, MarkWatched |
| Phase 4: Empty States | COMPLETE | Done in Part 1 Phase 5 |
| Phase 5: Distinguishing Features | PENDING | Time-based ordering, keyboard nav |

### Resolved Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Horizontal vs Vertical for Continue Watching | **Horizontal 16:9** | Industry standard, shows scene context, progress bar visible |
| Next Up Row | **Defer** | Include episode context in Continue Watching for now |
| BlurHash | **Defer** | Nice-to-have, not critical for v1 |
| Movie logos | **Fallback to text** | Only show if available |

### Phase 1: Continue Watching - Task Breakdown (COMPLETE)

**Backend:**
- [x] Modify `GetContinueWatchingFull()` to include progress data from `WatchProgress`
- [x] Add `MediaProgress` struct with percent, position_seconds, duration_seconds, remaining_text
- [x] Add `EpisodeContext` struct for TV shows (season, episode, title)
- [x] Add `ContinueWatchingItem` struct for richer response data
- [x] Create `internal/application/home/format.go` with `FormatRemainingTime()` and `CalculateProgressPercent()`
- [x] Create `internal/application/home/format_test.go` with unit tests
- [x] Update `getContinueWatchingData()` to return `items` array with progress
- [x] Include backdrop URL for each item

**Frontend:**
- [x] Create `ContinueWatchingCard.tsx` with horizontal 16:9 aspect ratio
- [x] Add progress bar component at bottom of card
- [x] Add "X min left" text display (remaining_text from backend)
- [x] Add episode badge (S2 E4) for TV shows
- [x] Add play button overlay on hover
- [x] Update `ContinueRow.tsx` to use `ContinueWatchingCard`
- [x] Update `widget.types.ts` with `ContinueWatchingItem`, `MediaProgress`, `EpisodeContext` types
- [x] Backward compatibility: falls back to MediaRow for legacy data format

### Phase 2: Hero Backdrop - Task Breakdown (COMPLETE)

**Backend:**
- [x] Add `HeroData` struct to `internal/domain/home/types.go`
- [x] Add `buildHeroData()` method to home service
- [x] Add `Hero` field to `HomeMeta` in response
- [x] Add `getGreeting()` helper for time-based greetings
- [x] Source backdrop from first continue watching or trending (with fallback)

**Frontend:**
- [x] Create `HeroBackdrop.tsx` component
- [x] Fetch backdrop image based on media type (movie/tv_show)
- [x] Apply blur, desaturation, gradient overlays
- [x] Add side vignette effect
- [x] Update `Home.tsx` to render hero backdrop
- [x] Add greeting text (Good morning/afternoon/evening)
- [x] Add date text (Saturday, January 4)
- [x] Add `HeroData` and `HomeMeta` types to widget.types.ts
- [x] Export `HeroBackdrop` from components/home

### Phase 3: Row Polish - Task Breakdown

**Frontend:**
- [ ] Create `NewBadge/NewBadge.tsx` component (7-day threshold)
- [ ] Create `NewBadge/index.ts` export
- [ ] Create `MarkWatchedButton/MarkWatchedButton.tsx` component
- [ ] Create `MarkWatchedButton/index.ts` export
- [ ] Update `MediaRow.tsx` header with item counts
- [ ] Apply `NewBadge` to recently added items
- [ ] Export new components from `media/index.ts`

### Phase 5: Distinguishing Features - Task Breakdown

**Backend:**
- [ ] Enhance `applySmartDefaults()` with time-based priority adjustments

**Frontend:**
- [ ] Add `onKeyDown` handler to `ScrollableRow`
- [ ] Implement Arrow Left/Right focus navigation
- [ ] Implement Enter to trigger click
- [ ] Add `focus-visible` ring styles to `MediaCard`
- [ ] Add `tabIndex={0}` to make cards focusable
- [ ] Add focus styles to `ContinueWatchingCard`

### Files to Create

| File | Phase | Purpose |
|------|-------|---------|
| `internal/application/home/format.go` | 1 | Time formatting helpers |
| `web/src/components/home/widgets/ContinueWatchingCard.tsx` | 1 | Horizontal card with progress |
| `web/src/components/home/HeroBackdrop.tsx` | 2 | Backdrop with blur/gradient |
| `web/src/components/media/NewBadge/NewBadge.tsx` | 3 | "New" indicator |
| `web/src/components/media/NewBadge/index.ts` | 3 | Export |
| `web/src/components/media/MarkWatchedButton/MarkWatchedButton.tsx` | 3 | Quick action |
| `web/src/components/media/MarkWatchedButton/index.ts` | 3 | Export |

### Files to Modify

| File | Phase | Changes |
|------|-------|---------|
| `internal/application/home/continue_watching.go` | 1 | Add progress data |
| `internal/application/home/service.go` | 1, 2 | Enriched data, hero data |
| `internal/domain/home/types.go` | 1, 2 | EpisodeContext, HeroData |
| `web/src/components/home/widgets/ContinueRow.tsx` | 1 | Use horizontal cards |
| `web/src/components/home/widgets/widget.types.ts` | 1 | ContinueWatchingItem type |
| `web/src/views/home/Home.tsx` | 2 | Hero backdrop, greeting |
| `web/src/components/home/widgets/MediaRow.tsx` | 3 | Counts, new badges |
| `web/src/components/media/index.ts` | 3 | New exports |
| `web/src/components/common/ScrollableRow/ScrollableRow.tsx` | 5 | Keyboard nav |
| `web/src/components/media/MediaCard/MediaCard.tsx` | 5 | Focus styles |
