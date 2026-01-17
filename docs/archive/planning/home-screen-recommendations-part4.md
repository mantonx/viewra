# Home Screen Enhancement Plan - Part 4

## Recommendation Systems Research

This document captures research on recommendation system libraries, algorithms, and best practices for enhancing ViewRA's personalized recommendations.

**Decision: Build our own collaborative filtering system** rather than depending on external libraries (disco-go) or services (Gorse). See [Final Decision](#final-decision-build-our-own) section for rationale.

---

## Current ViewRA Implementation Analysis

### Existing Architecture

The `recommendations` plugin (`plugins/recommendations/`) currently implements:

| Feature | Implementation | Location |
|---------|----------------|----------|
| **For You** | Get liked items → Find similar via semantic search → Fallback to genre | `recommendations.go:47` |
| **Because You Liked** | Pick a favorite → Find similar items | `recommendations.go:107` |
| **Similarity** | Vector search (semantic) or SQL genre matching | `recommendations.go:182` |

### Current Flow

```
User Request
    │
    ├─► Get positively rated items (favorites + upvotes)
    │
    ├─► If semantic search available:
    │       └─► FindSimilar() via PluginsClient → semantic-search plugin
    │
    └─► Fallback: Genre-based matching via ListMediaByGenre()
```

### Key Limitations

| Gap | Impact | Current Behavior |
|-----|--------|------------------|
| **No Collaborative Filtering** | Can't learn "users who watched X also watched Y" | Content-based only |
| **No Implicit Feedback** | Watch completion %, rewatch not used | Only explicit ratings |
| **No User Similarity** | Can't find users with similar tastes | No user-user patterns |
| **Poor Cold Start** | New users get nothing | Returns empty list |
| **No Exploration** | Always recommends similar content | No diversity/discovery |

---

## Library Options

### Option 1: Gorse (Full Recommendation Engine)

**Repository:** `github.com/gorse-io/gorse`  
**Stars:** 8.8k+ | **License:** Apache-2.0

#### Overview

Gorse is a production-ready, distributed recommendation engine written in Go. It runs as a separate service with REST API and Go SDK.

#### Architecture

```
┌─────────────┐     ┌─────────────────────────────────────┐
│   ViewRA    │────►│              Gorse                  │
│  (client)   │◄────│  ┌─────────┐  ┌─────────┐          │
└─────────────┘     │  │ Master  │  │ Workers │          │
                    │  │(training)│  │(offline)│          │
                    │  └────┬────┘  └────┬────┘          │
                    │       │            │               │
                    │  ┌────▼────────────▼────┐          │
                    │  │   Redis (cache)      │          │
                    │  │   PostgreSQL (data)  │          │
                    │  └──────────────────────┘          │
                    └─────────────────────────────────────┘
```

#### Features

- **Algorithms:** Matrix factorization (BPR, ALS), user-based CF, item-based CF
- **AutoML:** Automatically searches for best model hyperparameters
- **Multi-source:** Popular, latest, user-based, item-based, collaborative
- **Real-time:** Updates recommendations as feedback arrives
- **Scalable:** Horizontal scaling with worker nodes
- **Dashboard:** Web UI for monitoring and configuration

#### Integration Example

```go
import client "github.com/gorse-io/gorse-go"

// Initialize client
gorse := client.NewGorseClient("http://gorse:8088", "api_key")

// Push implicit feedback (watch events)
gorse.InsertFeedback([]client.Feedback{
    {
        FeedbackType: "watch",
        UserId:       userID,
        ItemId:       mediaID,
        Timestamp:    time.Now().Format(time.RFC3339),
    },
})

// Push explicit feedback (ratings)
gorse.InsertFeedback([]client.Feedback{
    {
        FeedbackType: "like",
        UserId:       userID,
        ItemId:       mediaID,
        Timestamp:    time.Now().Format(time.RFC3339),
    },
})

// Get personalized recommendations
recs, _ := gorse.GetRecommend(userID, "", 20)

// Get similar items (item-to-item)
similar, _ := gorse.GetItemNeighbors(itemID, "", 10)

// Get similar users
similarUsers, _ := gorse.GetUserNeighbors(userID, "", 5)
```

#### Configuration Example

```toml
# gorse.toml
[recommend]
positive_feedback_types = ["watch", "like", "favorite"]
read_feedback_types = ["watch"]

[recommend.offline]
enable_latest_recommend = true
enable_popular_recommend = true
enable_user_based_recommend = true
enable_item_based_recommend = true
enable_collaborative_recommend = true

[recommend.offline.explore_recommend]
popular = 0.1   # 10% exploration from popular
latest = 0.1    # 10% exploration from latest

[recommend.online]
fallback_recommend = ["item_based", "popular", "latest"]
num_feedback_fallback_item_based = 10
```

#### Pros/Cons

| Pros | Cons |
|------|------|
| Production-ready, battle-tested | Requires separate service |
| Full collaborative filtering | Adds operational complexity |
| AutoML reduces tuning | Another dependency to maintain |
| Active development | Overkill for small libraries |
| Good documentation | |

---

### Option 2: Disco-Go (Embedded Library) - NOT RECOMMENDED

**Repository:** `github.com/ankane/disco-go`  
**Stars:** 25 | **Forks:** 2 | **Watchers:** 1 | **License:** MIT

> **Warning:** Very low adoption. While the author (ankane) is reputable and maintains many popular libraries (including pgvector), the Go version has minimal community validation. The Ruby version (`ankane/disco`) has 600 stars and is more mature.

#### Overview

Disco-Go is a lightweight, embeddable recommendation library. It runs in-process with no external dependencies.

#### Features

- Matrix factorization with SGD (explicit) and conjugate gradient (implicit)
- User recommendations ("users like you also liked")
- Item recommendations ("users who liked this also liked")
- Similar users
- No external service required

#### Usage Example

```go
import "github.com/ankane/disco-go"

// Build dataset from interactions
data := disco.NewDataset[string, string]()

// Add implicit feedback (watches)
for _, watch := range watchHistory {
    data.Push(watch.UserID, watch.MediaID, 1.0)
}

// Add explicit feedback (ratings)
for _, rating := range ratings {
    data.Push(rating.UserID, rating.MediaID, rating.Score)
}

// Train model for implicit feedback
recommender, err := disco.FitImplicit(data, disco.Factors(50))

// Or for explicit feedback (ratings 1-5)
recommender, err := disco.FitExplicit(data, disco.Factors(50))

// Get recommendations for a user
recs := recommender.UserRecs(userID, 20)
for _, rec := range recs {
    fmt.Printf("Item: %s, Score: %.2f\n", rec.ID, rec.Score)
}

// Get similar items
similar := recommender.ItemRecs(itemID, 10)

// Get similar users
similarUsers := recommender.SimilarUsers(userID, 5)
```

#### Integration with ViewRA

```go
// In recommendations plugin
type RecommendationsService struct {
    // ... existing fields
    cfModel     *disco.Recommender[string, string]
    cfModelLock sync.RWMutex
    lastTrained time.Time
}

// Train model periodically (e.g., daily)
func (s *RecommendationsService) TrainModel(ctx context.Context) error {
    data := disco.NewDataset[string, string]()
    
    // Load watch history
    watches, _ := s.loadWatchHistory(ctx)
    for _, w := range watches {
        // Weight by completion percentage
        weight := float64(w.CompletionPercent) / 100.0
        data.Push(w.UserID, w.MediaID, weight)
    }
    
    // Load explicit ratings
    ratings, _ := s.loadAllRatings(ctx)
    for _, r := range ratings {
        score := 1.0
        if r.Rating == "favorite" {
            score = 2.0
        } else if r.Rating == "down" {
            score = -1.0
        }
        data.Push(r.UserID, r.MediaID, score)
    }
    
    // Train
    model, err := disco.FitImplicit(data, disco.Factors(50))
    if err != nil {
        return err
    }
    
    s.cfModelLock.Lock()
    s.cfModel = model
    s.lastTrained = time.Now()
    s.cfModelLock.Unlock()
    
    return nil
}

// Use model for recommendations
func (s *RecommendationsService) GetCollaborativeRecs(userID string, limit int) []string {
    s.cfModelLock.RLock()
    defer s.cfModelLock.RUnlock()
    
    if s.cfModel == nil {
        return nil
    }
    
    recs := s.cfModel.UserRecs(userID, limit)
    result := make([]string, len(recs))
    for i, r := range recs {
        result[i] = r.ID
    }
    return result
}
```

#### Pros/Cons

| Pros | Cons |
|------|------|
| Simple, embedded | Less feature-rich |
| No external dependencies | No distributed support |
| Fast for small-medium datasets | Must retrain periodically |
| MIT license | No built-in fallbacks |
| Easy to integrate | |

---

### Option 3: Hybrid with Existing Vector Search

#### Concept

Leverage the existing semantic-search plugin's vector infrastructure to add collaborative filtering signals.

#### How It Works

1. **Item embeddings** - Already generated from content (plot, genres, cast)
2. **User embeddings** - NEW: Average of watched/liked item embeddings
3. **Similarity search** - Use pgvector/sqlite-vec for fast retrieval

#### Implementation

```go
// Generate user taste profile as embedding
func (s *RecommendationsService) generateUserEmbedding(
    ctx context.Context, 
    userID string,
) ([]float32, error) {
    // Get user's positively interacted items
    likedIDs, _ := s.ratings.GetPositivelyRatedIDs(ctx, userID, "", 100)
    watchedIDs, _ := s.getHighCompletionWatches(ctx, userID, 100)
    
    allIDs := uniqueIDs(append(likedIDs, watchedIDs...))
    if len(allIDs) == 0 {
        return nil, ErrNoInteractions
    }
    
    // Collect embeddings
    var embeddings [][]float32
    for _, id := range allIDs {
        emb, err := s.plugins.GetEmbedding(ctx, "movie", id)
        if err == nil && emb != nil {
            embeddings = append(embeddings, emb.Vector)
        }
    }
    
    if len(embeddings) == 0 {
        return nil, ErrNoEmbeddings
    }
    
    // Average the embeddings to create user profile
    return averageVectors(embeddings), nil
}

// Find recommendations using user embedding
func (s *RecommendationsService) getVectorRecommendations(
    ctx context.Context,
    userID string,
    exclude map[int64]bool,
    limit int,
) ([]sdk.MediaItem, error) {
    // Generate user embedding
    userEmb, err := s.generateUserEmbedding(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // Search for items similar to user's taste profile
    results, err := s.plugins.VectorSearch(ctx, sdk.VectorSearchRequest{
        QueryVector:   userEmb,
        EntityTypes:   []string{"movie", "tv_show"},
        Limit:         limit * 2, // Get extra to filter
        MinSimilarity: 0.5,
    })
    if err != nil {
        return nil, err
    }
    
    // Filter out already watched/rated
    var items []sdk.MediaItem
    for _, r := range results.Results {
        if !exclude[r.EntityID] {
            items = append(items, sdk.MediaItem{
                EntityType: r.EntityType,
                EntityID:   r.EntityID,
                Reason:     "Matches your taste",
            })
        }
        if len(items) >= limit {
            break
        }
    }
    
    return items, nil
}

// Helper: average multiple vectors
func averageVectors(vectors [][]float32) []float32 {
    if len(vectors) == 0 {
        return nil
    }
    
    dims := len(vectors[0])
    result := make([]float32, dims)
    
    for _, v := range vectors {
        for i := range result {
            result[i] += v[i]
        }
    }
    
    n := float32(len(vectors))
    for i := range result {
        result[i] /= n
    }
    
    return result
}
```

#### Pros/Cons

| Pros | Cons |
|------|------|
| Uses existing infrastructure | DIY implementation |
| No new dependencies | Less sophisticated |
| Combines content + collaborative | Requires tuning |
| Leverages pgvector investment | |

---

### Option 4: mlpack-go (Scientific ML)

**Repository:** `github.com/mlpack/mlpack-go`  
**License:** BSD-3

#### Overview

Go bindings for the mlpack C++ library. Research-grade algorithms including SVD, NMF, BiasSVD, SVD++.

#### Pros/Cons

| Pros | Cons |
|------|------|
| Research-grade algorithms | Requires CGO |
| Many CF variants | C++ dependency |
| | Complex deployment |
| | Large binary size |

**Recommendation:** Not ideal for ViewRA due to CGO complexity.

---

## Reference Implementations

### Microsoft Recommenders

**Repository:** `recommenders-team/recommenders`  
**Stars:** 21,300+ | **License:** MIT | **Maintainer:** Linux Foundation AI & Data

#### Overview

Microsoft Recommenders is a comprehensive collection of recommendation system algorithms, examples, and best practices. While it's a Python library (not directly usable in Go), it's an invaluable educational resource with well-documented algorithms and Jupyter notebooks.

#### Key Algorithms

| Algorithm | Type | Training Required | Complexity | Best For |
|-----------|------|-------------------|------------|----------|
| **SAR** | Item-based CF | No (matrix ops only) | Low | Quick wins, interpretable |
| **BPR** | Matrix factorization | Yes (SGD) | Medium | Implicit feedback |
| **NCF** | Neural CF | Yes (deep learning) | High | Large scale, complex patterns |
| **LightGCN** | Graph neural network | Yes | High | Social/interaction graphs |

#### SAR: Simple Algorithm for Recommendation

SAR is a Microsoft-invented algorithm that's surprisingly effective without any ML training. It's ideal for a first CF implementation.

**How SAR Works:**

```
Step 1: Build Item-Item Similarity Matrix
        - Count co-occurrences: how often items appear together in user histories
        - Normalize by item popularity (Jaccard similarity)

Step 2: Build User-Item Affinity Matrix  
        - For each user, record which items they interacted with
        - Apply time decay: recent interactions weighted higher

Step 3: Generate Recommendations
        - Recommendations = Affinity × Similarity
        - For user U: score(item I) = Σ affinity(U, J) × similarity(J, I)
                                      for all items J user interacted with
```

**SAR Implementation Sketch:**

```go
// SAR doesn't require gradient descent - just matrix operations
type SARModel struct {
    // Item-item similarity: similarity[itemA][itemB] = jaccard coefficient
    similarity map[int64]map[int64]float32
    
    // User-item affinity: affinity[userID][itemID] = weighted interaction score
    affinity map[string]map[int64]float32
    
    // Item popularity for normalization
    itemCounts map[int64]int
}

// Build similarity matrix from co-occurrence
func (m *SARModel) BuildSimilarity(interactions []Interaction) {
    // Count co-occurrences: items that appear in same user's history
    cooccurrence := make(map[int64]map[int64]int)
    
    // Group interactions by user
    userItems := groupByUser(interactions)
    
    for _, items := range userItems {
        // For each pair of items in user's history
        for i := 0; i < len(items); i++ {
            for j := i + 1; j < len(items); j++ {
                // Increment co-occurrence count
                cooccurrence[items[i]][items[j]]++
                cooccurrence[items[j]][items[i]]++
            }
        }
    }
    
    // Normalize by item popularity (Jaccard: intersection / union)
    for itemA, neighbors := range cooccurrence {
        for itemB, count := range neighbors {
            countA := m.itemCounts[itemA]
            countB := m.itemCounts[itemB]
            // Jaccard similarity
            m.similarity[itemA][itemB] = float32(count) / float32(countA+countB-count)
        }
    }
}

// Build user affinity with time decay
func (m *SARModel) BuildAffinity(interactions []Interaction, halfLife time.Duration) {
    now := time.Now()
    
    for _, inter := range interactions {
        age := now.Sub(inter.Timestamp)
        // Exponential time decay
        weight := math.Pow(0.5, float64(age)/float64(halfLife))
        
        if m.affinity[inter.UserID] == nil {
            m.affinity[inter.UserID] = make(map[int64]float32)
        }
        m.affinity[inter.UserID][inter.ItemID] += float32(weight)
    }
}

// Generate recommendations: affinity × similarity
func (m *SARModel) Recommend(userID string, limit int, exclude map[int64]bool) []int64 {
    userAffinity := m.affinity[userID]
    if len(userAffinity) == 0 {
        return nil // Cold start
    }
    
    scores := make(map[int64]float32)
    
    // For each item user has affinity with
    for itemJ, affinity := range userAffinity {
        // Add weighted similarity to all similar items
        for itemI, sim := range m.similarity[itemJ] {
            if !exclude[itemI] {
                scores[itemI] += affinity * sim
            }
        }
    }
    
    return topK(scores, limit)
}
```

**SAR vs BPR Comparison:**

| Aspect | SAR | BPR |
|--------|-----|-----|
| Training | Matrix operations only | SGD optimization |
| Lines of code | ~150 | ~250 |
| Interpretability | High (similarity scores) | Low (latent factors) |
| Cold start items | Handles via similarity | Needs some interactions |
| Scalability | O(items²) similarity matrix | O(factors × items) |
| Accuracy | Good | Better |
| Implementation time | 1-2 days | 3-5 days |

**Recommendation:** Implement SAR first (Phase 1.5) as it's simpler, interpretable, and provides immediate value. Then add BPR (Phase 2) for improved accuracy.

#### Key Notebooks to Reference

| Notebook | Purpose |
|----------|---------|
| `examples/00_quick_start/sar_movielens.ipynb` | SAR implementation walkthrough |
| `examples/02_model_collaborative_filtering/cornac_bpr_deep_dive.ipynb` | BPR algorithm deep dive |
| `examples/02_model_collaborative_filtering/als_deep_dive.ipynb` | ALS for explicit ratings |

---

### Twitter/X Algorithm

**Repository:** `twitter/the-algorithm`  
**Stars:** 69,200+ | **License:** AGPL-3.0

#### Overview

Twitter open-sourced their recommendation algorithm in 2023. While it's a massive Scala/Java system designed for 500M+ users, several concepts are applicable at any scale.

#### Architecture: Two-Phase Ranking

Twitter's home timeline uses a two-phase approach that's widely adopted in industry:

```
Phase 1: Candidate Generation (~1500 tweets)
├── In-Network: Tweets from followed accounts
├── Out-of-Network: Tweets from non-followed accounts
│   ├── Social Graph: Friends-of-friends, similar users
│   └── SimClusters: Community-based similarity
└── Filters: Quality, safety, dedup

Phase 2: Heavy Ranking (~300 tweets)
├── ML Model: ~48k features per tweet
├── Engagement prediction: P(like), P(retweet), P(reply)
├── Score = weighted sum of predictions
└── Final ranking with diversity injection
```

**Applicability to ViewRA:**

| Twitter Concept | ViewRA Adaptation |
|-----------------|-------------------|
| Candidate generation | Get 100+ candidates from CF + semantic + popular |
| Heavy ranking | Score by user preference model (simpler than 48k features) |
| Diversity injection | Ensure genre/era variety in final list |
| In-network | Previously watched → continue watching |
| Out-of-network | Discovery via CF and exploration |

#### SimClusters: Community-Based Embeddings

SimClusters is Twitter's approach to user/content embeddings based on community membership rather than learned latent factors.

**How SimClusters Works:**

```
1. Cluster users into ~145,000 communities based on follow graph
   - Communities are interpretable: "NBA fans", "Tech enthusiasts", etc.
   
2. Build KnownFor matrix (producers → communities)
   - For each content creator, which communities follow them?
   - Sparse vector: most users belong to ~10-50 communities
   
3. Build InterestedIn matrix (consumers → communities)
   - For each user, which communities do they engage with?
   
4. Tweet embeddings = aggregation of engager community vectors
   - When users engage with a tweet, their community vectors contribute
   - Real-time updates on each like/retweet
   
5. Recommendations = cosine similarity between user and content vectors
```

**SimClusters Advantages:**

| Advantage | Description |
|-----------|-------------|
| **Interpretable** | Can explain: "Recommended because you like Sci-Fi" |
| **Sparse** | 145k dimensions but ~50 non-zero = efficient storage |
| **Real-time** | Tweet embeddings update immediately on engagement |
| **Cold start** | New content gets embedding from first engagers |

**ViewRA Adaptation - Genre/Tag Clusters:**

Instead of 145k communities, ViewRA could use a simpler approach:

```go
// Community = Genre/Tag combination
// ~100-500 "communities" based on metadata

type UserCommunityProfile struct {
    UserID     string
    // Sparse vector: community -> affinity score
    // e.g., {"sci-fi-action": 0.8, "drama-romance": 0.3, ...}
    Communities map[string]float32
}

// Build from watch history
func buildUserProfile(watches []WatchHistory) UserCommunityProfile {
    profile := UserCommunityProfile{Communities: make(map[string]float32)}
    
    for _, w := range watches {
        // Weight by completion
        weight := float32(w.CompletionPercent) / 100.0
        
        // Add to each genre community
        for _, genre := range w.Genres {
            profile.Communities[genre] += weight
        }
        
        // Add to combined communities (more specific)
        if len(w.Genres) >= 2 {
            combo := strings.Join(w.Genres[:2], "-")
            profile.Communities[combo] += weight * 0.5
        }
    }
    
    // Normalize
    normalize(profile.Communities)
    return profile
}

// Score item by community overlap
func scoreItem(userProfile UserCommunityProfile, item MediaItem) float32 {
    var score float32
    for _, genre := range item.Genres {
        score += userProfile.Communities[genre]
    }
    return score
}
```

#### Lessons for ViewRA

| Lesson | Application |
|--------|-------------|
| **Two-phase ranking** | Generate candidates broadly, then re-rank by user preference |
| **Sparse interpretable vectors** | Use genre/tag profiles instead of opaque embeddings |
| **Real-time updates** | Update user profile on each watch completion |
| **Diversity** | Ensure recommendations span multiple genres/eras |
| **Explain recommendations** | "Because you watched Sci-Fi" based on community overlap |

---

## Industry Best Practices

### Netflix Approach

1. **Multiple algorithms** - 100+ models for different contexts
2. **Row-based UI** - Each home row is a different algorithm
3. **Two-phase ranking:**
   - **Candidate generation** - Fast, broad retrieval
   - **Ranking** - ML model scores by likelihood to watch
4. **Context-aware** - Time of day, device, recent activity

### Implicit Feedback Weighting

| Signal | Weight | Rationale |
|--------|--------|-----------|
| Watch completion >90% | High positive | Finished = liked |
| Watch completion 50-90% | Medium positive | Engaged |
| Watch completion <10% | Weak negative | Abandoned early |
| Rewatch | Very high positive | Really liked |
| Skip/dismiss | Negative | Not interested |
| Explicit like | High positive | Clear signal |
| Explicit favorite | Very high positive | Strongest signal |
| Explicit dislike | High negative | Never show again |

### Cold Start Strategies

| Scenario | Solution |
|----------|----------|
| **New user** | Popular items, trending, editorial picks |
| **New item** | Content-based (genres, cast), show to exploratory users |
| **Both new** | Global popularity, genre defaults |

### Exploration vs. Exploitation

Balance familiar recommendations with discovery:

```
Recommendations = 
    80% Exploitation (similar to what user likes)
  + 10% Popular items (social proof)
  + 10% New/Latest items (freshness)
```

---

## Comparison Matrix

| Feature | Current | Gorse | Disco-Go | Vector Hybrid |
|---------|---------|-------|----------|---------------|
| **Complexity** | Low | High | Low | Medium |
| **External Service** | No | Yes | No | No |
| **Collaborative Filtering** | No | Yes | Yes | Partial |
| **Implicit Feedback** | No | Yes | Yes | Yes |
| **Cold Start Handling** | Poor | Good | Manual | Manual |
| **Real-time Updates** | N/A | Yes | Batch | Real-time |
| **Scalability** | N/A | High | Medium | High |
| **Maintenance** | Low | High | Low | Medium |

---

## Recommended Approach

### Phase 1: Quick Wins (Enhance Existing)

**Effort:** 1-2 days

1. **Add cold start fallback**
   ```go
   if len(likedIDs) == 0 {
       // Return popular/trending instead of empty
       return s.getPopularItems(ctx, limit)
   }
   ```

2. **Mix in exploration**
   ```go
   // 80% personalized, 20% exploration
   personalizedCount := limit * 80 / 100
   explorationCount := limit - personalizedCount
   
   personalized := s.getPersonalizedRecs(ctx, userID, personalizedCount)
   trending := s.getTrendingItems(ctx, explorationCount)
   
   return shuffle(append(personalized, trending...))
   ```

3. **Add implicit feedback** (if watch progress is accessible)
   ```go
   // Consider high-completion watches as positive signals
   highCompletionWatches := s.getWatchesAboveThreshold(ctx, userID, 80)
   likedIDs = append(likedIDs, highCompletionWatches...)
   ```

### Phase 2: User Embeddings (Vector Hybrid)

**Effort:** 2-3 days

1. Generate user embeddings from interaction history
2. Store in vector database alongside item embeddings
3. Use for "users with similar taste" recommendations

### Phase 3: Disco-Go Integration

**Effort:** 3-5 days

1. Add disco-go dependency
2. Train model on startup and periodically
3. Combine CF results with semantic search
4. Store model in plugin data directory (using Files SDK)

### Phase 4: Gorse Integration (Optional)

**Effort:** 1-2 weeks

1. Add Gorse as optional sidecar service
2. Push all interactions to Gorse
3. Use Gorse API for recommendations
4. Keep semantic search as fallback

---

## Implementation Plan

### Files to Modify

| File | Changes |
|------|---------|
| `plugins/recommendations/internal/recommendations.go` | Add cold start, exploration, implicit feedback |
| `plugins/recommendations/internal/collaborative.go` | NEW: Disco-Go integration |
| `plugins/recommendations/internal/user_embedding.go` | NEW: User embedding generation |
| `plugins/recommendations/go.mod` | Add disco-go dependency |

### New Types

```go
// UserProfile represents a user's taste profile
type UserProfile struct {
    UserID        string
    Embedding     []float32  // Average of liked item embeddings
    TopGenres     []string   // Most watched genres
    LastUpdated   time.Time
}

// ImplicitFeedback represents watch behavior
type ImplicitFeedback struct {
    UserID            string
    MediaID           int64
    MediaType         string
    CompletionPercent int
    WatchCount        int
    LastWatched       time.Time
}
```

### Configuration

```yaml
# config.yml additions
recommendations:
  enabled: true
  max_recommendations: 20
  
  # Recommendation sources
  collaborative_filtering: true
  content_based: true
  
  # Exploration/exploitation balance
  exploration_ratio: 0.2  # 20% exploration
  
  # Implicit feedback settings
  use_watch_history: true
  completion_threshold: 80  # Consider >80% as positive signal
  
  # Cold start
  cold_start_fallback: "popular"  # popular, trending, or latest
  
  # Model training (if using Disco-Go)
  train_interval: "24h"
  min_interactions: 10  # Minimum interactions before training
```

---

## Open Questions

1. **Watch history access** - Can the recommendations plugin access watch progress data? Currently it only has access to explicit ratings via `RatingsClient`.

2. **Training frequency** - How often should CF models be retrained? Daily? On-demand?

3. **External service tolerance** - Is running Gorse as a separate service acceptable, or must everything be embedded?

4. **Scale expectations** - Expected number of users and media items? Affects algorithm choice.

---

## Summary

| Approach | Best For | Effort |
|----------|----------|--------|
| **Quick Wins** | Immediate improvement | 1-2 days |
| **Vector Hybrid** | Leverage existing infrastructure | 2-3 days |
| **Disco-Go** | Real collaborative filtering, no external service | 3-5 days |
| **Gorse** | Production scale, full features | 1-2 weeks |

**Recommendation:** Build our own collaborative filtering system. See implementation plan below.

---

## Final Decision: Build Our Own

After analyzing both Gorse and Disco-Go, we've decided to build our own collaborative filtering system tailored to ViewRA's needs.

### Library Analysis Summary

| Library | Stars | Status | Verdict |
|---------|-------|--------|---------|
| **Gorse** | 9,274 | Healthy, active | Overkill - requires external service, Redis, operational complexity |
| **Disco-Go** | 25 | Low adoption risk | Too risky - minimal community, could be abandoned |

### Why Build Our Own

1. **We already have vector infrastructure** - pgvector/sqlite-vec for storing and searching embeddings
2. **The core algorithm is simple** - BPR matrix factorization is ~200-300 lines of Go
3. **No dependency risk** - disco-go could be abandoned (25 stars, 2 forks)
4. **No operational overhead** - Gorse requires running a separate service + Redis
5. **Domain-specific tuning** - Can optimize for media (weight watch completion, handle episodes vs movies)
6. **Seamless integration** - Combines naturally with existing semantic search

### What Gorse Provides vs. What We Need

| Gorse Feature | We Need? | Our Solution |
|---------------|----------|--------------|
| Matrix Factorization (BPR) | Yes | Build (~200 lines) |
| Item Similarity | Already have | semantic-search plugin |
| User Similarity | Optional | Simple to add (~50 lines) |
| Popular/Trending | Already have | Existing widgets |
| Factor Storage | Yes | pgvector (existing) |
| AutoML | No | Sensible defaults, manual tuning |
| Distributed Workers | No | Single-server is fine |
| Dashboard | No | ViewRA admin panel |
| REST API | Already have | Plugin routes |

### Core Algorithm: BPR (Bayesian Personalized Ranking)

The heart of collaborative filtering for implicit feedback:

```go
// For each training triple (user, liked_item, disliked_item):
func (m *BPRModel) trainStep(user, posItem, negItem string) {
    userVec := m.userFactors[user]
    posVec := m.itemFactors[posItem]
    negVec := m.itemFactors[negItem]
    
    // Prediction: user should prefer posItem over negItem
    posPred := dot(userVec, posVec)
    negPred := dot(userVec, negVec)
    diff := posPred - negPred
    
    // Sigmoid gradient for ranking loss
    grad := 1.0 / (1.0 + math.Exp(diff))
    
    // Update factors via gradient descent
    for k := 0; k < m.factors; k++ {
        userGrad := grad * (posVec[k] - negVec[k])
        posGrad := grad * userVec[k]
        negGrad := -grad * userVec[k]
        
        m.userFactors[user][k] += m.lr * (userGrad - m.reg*userVec[k])
        m.itemFactors[posItem][k] += m.lr * (posGrad - m.reg*posVec[k])
        m.itemFactors[negItem][k] += m.lr * (negGrad - m.reg*negVec[k])
    }
}
```

### Factor Storage with pgvector

Store learned factors in existing vector infrastructure for fast similarity search:

```go
// Store factors alongside semantic embeddings
func (m *CFModel) SaveFactors(ctx context.Context, vectorClient *sdk.VectorClient) error {
    // User factors - separate namespace from content embeddings
    for userID, vec := range m.userFactors {
        vectorClient.Store(ctx, sdk.Embedding{
            EntityType: "cf_user",
            EntityID:   hashUserID(userID),
            Vector:     vec,
        })
    }
    
    // Item factors
    for itemID, vec := range m.itemFactors {
        vectorClient.Store(ctx, sdk.Embedding{
            EntityType: "cf_item",
            EntityID:   itemID,
            Vector:     vec,
        })
    }
    return nil
}

// Fast recommendation using vector similarity
func (m *CFModel) Recommend(ctx context.Context, userID string, limit int) ([]int64, error) {
    userFactor := m.userFactors[userID]
    
    // Use pgvector for efficient similarity search
    results, _ := m.vectorClient.Search(ctx, sdk.VectorSearchRequest{
        QueryVector: userFactor,
        EntityTypes: []string{"cf_item"},
        Limit:       limit,
    })
    
    return extractIDs(results), nil
}
```

---

## Revised Implementation Plan

### Architecture Overview

```
plugins/recommendations/internal/
├── cf/
│   ├── bpr.go           # BPR training algorithm
│   ├── model.go         # Factor storage/retrieval  
│   ├── trainer.go       # Training job runner
│   └── sampler.go       # Negative sampling strategies
├── hybrid.go            # Combine CF + semantic + popular
├── user_embedding.go    # User embedding from liked items
└── recommendations.go   # Enhanced with new strategies
```

### Phase 1: User Embedding Average (Immediate)

**Effort:** 1-2 days  
**Impact:** Medium - works immediately, no training required

Use existing semantic embeddings to create user taste profiles:

```go
// User embedding = weighted average of liked item embeddings
func (s *Service) generateUserEmbedding(ctx context.Context, userID string) ([]float32, error) {
    likedIDs, _ := s.ratings.GetPositivelyRatedIDs(ctx, userID, "", 100)
    
    var embeddings [][]float32
    var weights []float32
    
    for i, id := range likedIDs {
        emb, _ := s.getItemEmbedding(ctx, id)
        if emb != nil {
            embeddings = append(embeddings, emb)
            // More recent = higher weight
            weights = append(weights, 1.0 / float32(i+1))
        }
    }
    
    return weightedAverage(embeddings, weights), nil
}
```

### Phase 1.5: SAR Collaborative Filtering (NEW)

**Effort:** 1-2 days  
**Impact:** High - true collaborative patterns without ML training

Implement SAR (Simple Algorithm for Recommendation) as a simpler alternative to BPR. SAR uses item co-occurrence patterns without gradient descent.

```go
// SAR model - no training required, just matrix operations
type SARModel struct {
    similarity map[int64]map[int64]float32 // Item-item Jaccard similarity
    affinity   map[string]map[int64]float32 // User-item affinity with time decay
}

// Build from interaction history
func (m *SARModel) Build(interactions []Interaction, halfLife time.Duration) {
    // Step 1: Count co-occurrences
    userItems := groupByUser(interactions)
    cooccur := make(map[int64]map[int64]int)
    itemCounts := make(map[int64]int)
    
    for _, items := range userItems {
        for _, item := range items {
            itemCounts[item]++
        }
        for i := 0; i < len(items); i++ {
            for j := i + 1; j < len(items); j++ {
                incr(cooccur, items[i], items[j])
                incr(cooccur, items[j], items[i])
            }
        }
    }
    
    // Step 2: Jaccard similarity
    for a, neighbors := range cooccur {
        for b, count := range neighbors {
            m.similarity[a][b] = float32(count) / float32(itemCounts[a]+itemCounts[b]-count)
        }
    }
    
    // Step 3: User affinity with time decay
    now := time.Now()
    for _, inter := range interactions {
        weight := math.Pow(0.5, float64(now.Sub(inter.Time))/float64(halfLife))
        m.affinity[inter.UserID][inter.ItemID] += float32(weight)
    }
}

// Recommend: affinity × similarity
func (m *SARModel) Recommend(userID string, limit int, exclude map[int64]bool) []int64 {
    scores := make(map[int64]float32)
    for itemJ, aff := range m.affinity[userID] {
        for itemI, sim := range m.similarity[itemJ] {
            if !exclude[itemI] {
                scores[itemI] += aff * sim
            }
        }
    }
    return topK(scores, limit)
}
```

**Why SAR Before BPR:**

| Reason | Explanation |
|--------|-------------|
| **Simpler** | ~150 lines vs ~250 for BPR |
| **No training** | Matrix operations only, no SGD |
| **Interpretable** | Can explain: "Similar to items you watched" |
| **Fast iteration** | Debug and tune quickly |
| **Good baseline** | If SAR works well, BPR may be unnecessary |

### Phase 2: BPR Collaborative Filtering

**Effort:** 3-5 days  
**Impact:** High - true collaborative patterns

Build matrix factorization with BPR:

| Component | Lines | Description |
|-----------|-------|-------------|
| `bpr.go` | ~200 | Core BPR algorithm |
| `model.go` | ~100 | Factor management |
| `trainer.go` | ~100 | Training orchestration |
| `sampler.go` | ~50 | Negative sampling |
| **Total** | ~450 | |

### Phase 3: Hybrid Scoring

**Effort:** 1-2 days  
**Impact:** High - best of all worlds

Combine multiple signals:

```go
func (s *Service) GetHybridRecommendations(ctx context.Context, userID string, limit int) []MediaItem {
    // 50% collaborative filtering (learned patterns)
    cfRecs := s.getCollaborativeRecs(ctx, userID, limit*50/100)
    
    // 30% semantic similarity (content-based)
    semanticRecs := s.getSemanticRecs(ctx, userID, limit*30/100)
    
    // 20% exploration (popular/trending/new)
    exploration := s.getExplorationRecs(ctx, limit*20/100)
    
    return mergeAndDeduplicate(cfRecs, semanticRecs, exploration)
}
```

### Phase 4: Enhancements

**Effort:** 2-3 days  
**Impact:** Medium - polish and optimization

- Cold start handling (popular items for new users)
- Implicit feedback integration (watch completion %)
- Periodic retraining via scheduler
- Model persistence using Files SDK

---

## Files to Create

| File | Purpose | Lines (est.) |
|------|---------|--------------|
| `plugins/recommendations/internal/cf/sar.go` | SAR algorithm (Phase 1.5) | ~150 |
| `plugins/recommendations/internal/cf/bpr.go` | BPR algorithm (Phase 2) | ~200 |
| `plugins/recommendations/internal/cf/model.go` | Factor storage | ~100 |
| `plugins/recommendations/internal/cf/trainer.go` | Training runner | ~100 |
| `plugins/recommendations/internal/cf/sampler.go` | Negative sampling | ~50 |
| `plugins/recommendations/internal/hybrid.go` | Score combination | ~100 |
| `plugins/recommendations/internal/user_embedding.go` | User profiles | ~80 |

## Files to Modify

| File | Changes |
|------|---------|
| `plugins/recommendations/internal/recommendations.go` | Use hybrid scoring, add cold start |
| `plugins/recommendations/internal/plugin.go` | Initialize CF model, schedule training |
| `plugins/recommendations/internal/types.go` | Add CF-related types |

---

## Training Data Requirements

### Implicit Feedback Sources

| Source | Signal | Weight |
|--------|--------|--------|
| Watch completion >90% | Strong positive | 1.0 |
| Watch completion 50-90% | Positive | 0.7 |
| Watch completion <20% | Weak negative | -0.3 |
| Rewatch | Very strong positive | 1.5 |
| Explicit favorite | Strongest positive | 2.0 |
| Explicit upvote | Strong positive | 1.0 |
| Explicit downvote | Strong negative | -1.0 |

### Negative Sampling

For BPR, we need (user, positive_item, negative_item) triples:

```go
// Sample negative items the user hasn't interacted with
func (s *Sampler) SampleNegative(userID string, positiveItems []int64) int64 {
    for {
        // Random item from catalog
        candidate := s.randomItem()
        
        // Must not be in user's positive set
        if !contains(positiveItems, candidate) {
            return candidate
        }
    }
}
```

---

## Model Hyperparameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `factors` | 50 | Embedding dimension |
| `learning_rate` | 0.01 | SGD learning rate |
| `regularization` | 0.01 | L2 regularization |
| `epochs` | 20 | Training iterations |
| `negative_samples` | 4 | Negatives per positive |

---

## Open Questions (Resolved)

| Question | Answer |
|----------|--------|
| Use external library? | **No** - build our own |
| Use external service (Gorse)? | **No** - too much overhead |
| Training frequency? | Nightly via existing scheduler |
| Factor storage? | pgvector (existing infrastructure) |

---

## Estimated Total Effort

| Phase | Effort | Cumulative |
|-------|--------|------------|
| Phase 1: User Embeddings | 1-2 days | 1-2 days |
| Phase 1.5: SAR CF | 1-2 days | 2-4 days |
| Phase 2: BPR CF (optional) | 3-5 days | 5-9 days |
| Phase 3: Hybrid Scoring | 1-2 days | 6-11 days |
| Phase 4: Enhancements | 2-3 days | 8-14 days |

**Minimum viable: 4-6 days** (Phases 1, 1.5, 3) - User embeddings + SAR + hybrid scoring  
**Full implementation: 8-14 days** for a complete, custom collaborative filtering system that:
- Integrates seamlessly with existing semantic search
- Uses existing vector infrastructure (pgvector)
- Has no external dependencies
- Is tailored for media recommendations

---

## References

### Algorithms & Research

| Resource | URL | Notes |
|----------|-----|-------|
| Microsoft Recommenders | `github.com/recommenders-team/recommenders` | SAR, BPR, NCF implementations |
| SAR Notebook | `recommenders/.../sar_movielens.ipynb` | SAR walkthrough on MovieLens |
| BPR Deep Dive | `recommenders/.../cornac_bpr_deep_dive.ipynb` | BPR algorithm explanation |
| Twitter Algorithm | `github.com/twitter/the-algorithm` | Production recommendation system |
| SimClusters Paper | KDD 2020 | Community-based embeddings |

### Key Papers

| Paper | Year | Relevance |
|-------|------|-----------|
| BPR: Bayesian Personalized Ranking | 2009 | Foundation for implicit feedback CF |
| Matrix Factorization for Recommender Systems | 2009 | Netflix Prize winning approach |
| Deep Learning based Recommender System | 2017 | Neural collaborative filtering |

### Implementation Notes

**SAR vs BPR Decision Tree:**

```
Start with SAR if:
├── < 100k items
├── Need quick iteration
├── Want interpretable results
└── Accuracy is "good enough"

Add BPR if:
├── SAR accuracy insufficient
├── Have GPU for training
├── Need better personalization
└── Willing to invest 3-5 more days
```

**Two-Phase Ranking (from Twitter):**

```
Apply to ViewRA:
1. Candidate Generation (fast, broad)
   - SAR/BPR: 50 candidates
   - Semantic search: 30 candidates  
   - Popular/trending: 20 candidates
   
2. Re-ranking (slower, precise)
   - Score by user genre profile
   - Apply diversity constraints
   - Filter recently watched
   - Return top 20
```

**Future Enhancements (from SimClusters):**

- Genre/tag-based community profiles (sparse, interpretable)
- Real-time profile updates on watch completion
- Explainable recommendations: "Because you like Sci-Fi"

---

## Implementation Progress

### Status Summary

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1: User Embedding Average | PENDING | 1-2 days - no training required |
| Phase 1.5: SAR Collaborative Filtering | PENDING | 1-2 days - matrix operations only |
| Phase 2: BPR Collaborative Filtering | DEFERRED | 3-5 days - implement if SAR insufficient |
| Phase 3: Hybrid Scoring | PENDING | 1-2 days - combine all signals |
| Phase 4: Enhancements | PENDING | 2-3 days - cold start, persistence |

### Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| SAR vs BPR first | **SAR first** | Simpler (~150 lines), no ML training, faster iteration |
| BPR implementation | **Deferred** | Implement only if SAR quality insufficient |
| HostProgress service | **Yes** | Needed for SAR to access watch history from plugin |
| Unit tests | **Yes** | Test SAR, hybrid scoring, edge cases |

### Infrastructure Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| HostProgress gRPC service | PENDING | New service like HostRatings |
| ProgressClient in SDK | PENDING | `pkg/plugin/sdk/progress.go` |
| Plugin Files SDK | EXISTS | For model persistence |
| pgvector | EXISTS | For factor storage if using BPR |

### Phase 1: User Embedding Average - Task Breakdown

**Files to Create:**
- [ ] `plugins/recommendations/internal/user_embedding.go`
  - `UserEmbeddingService` struct
  - `GenerateUserEmbedding()` - weighted average of liked item embeddings
  - `weightedAverage()` - helper for combining embeddings

**Files to Modify:**
- [ ] `plugins/recommendations/internal/recommendations.go` - add `getUserEmbeddingRecommendations()`
- [ ] `plugins/recommendations/internal/plugin.go` - wire UserEmbeddingService

**Tests:**
- [ ] `plugins/recommendations/internal/user_embedding_test.go`

### Phase 1.5: SAR - Task Breakdown

**Infrastructure:**
- [ ] Add HostProgress proto messages to `api/proto/plugin/host_services.proto`
  - `ListWatchHistoryRequest`, `ListWatchHistoryResponse`, `WatchHistoryItem`
- [ ] Create `internal/infrastructure/plugins/host/progress.go` - HostProgress server
- [ ] Create `pkg/plugin/sdk/progress.go` - ProgressClient for plugins
- [ ] Wire HostProgress in `manager/loader.go`, `grpc/factory.go`
- [ ] Add `host_progress_broker_id` to `InitRequest` proto
- [ ] Update SDK `HostServices` with `Progress` field

**SAR Implementation:**
- [ ] Create `plugins/recommendations/internal/cf/types.go`
  - `Interaction` struct (UserID, ItemID, Timestamp, Weight)
- [ ] Create `plugins/recommendations/internal/cf/sar.go`
  - `SARModel` struct (similarity, affinity, itemCounts maps)
  - `NewSARModel()` constructor
  - `Build()` - compute item-item Jaccard similarity + user-item affinity
  - `Recommend()` - score items by affinity × similarity
  - `topK()` helper

**Integration:**
- [ ] Add `getSARRecommendations()` to `recommendations.go`
- [ ] Initialize SAR model in plugin startup
- [ ] Add scheduled rebuild (nightly)

**Tests:**
- [ ] `plugins/recommendations/internal/cf/sar_test.go`
  - Test with known interactions
  - Test cold start (no history)
  - Test exclusion of already-watched items

### Phase 3: Hybrid Scoring - Task Breakdown

**Files to Create:**
- [ ] `plugins/recommendations/internal/hybrid.go`
  - `HybridScorer` struct with configurable weights
  - `Score()` - combine CF + semantic + exploration
  - `mergeAndDeduplicate()` - combine sources, remove duplicates

**Integration:**
- [ ] Update `GetForYou()` in `recommendations.go` to use hybrid scoring
- [ ] Add fallback chain: hybrid → semantic → genre → popular

**Tests:**
- [ ] `plugins/recommendations/internal/hybrid_test.go`
  - Test weight distribution
  - Test deduplication
  - Test fallback behavior

### Phase 4: Enhancements - Task Breakdown

**Cold Start:**
- [ ] Add `getPopularItems()` method for new users
- [ ] Return trending/popular when no ratings or history

**Implicit Feedback:**
- [ ] Weight watch completion in SAR affinity scores
  - >90% completion: +1.0
  - 50-90%: +0.7
  - <20%: -0.3
  - Rewatch: +1.5
- [ ] Weight explicit ratings higher
  - Favorite: 2.0, Upvote: 1.0, Downvote: -1.0

**Model Persistence:**
- [ ] Save SAR model to plugin data directory
- [ ] Load on plugin startup
- [ ] Track last rebuild timestamp
- [ ] Scheduled nightly rebuild

### Files to Create (Summary)

| File | Phase | Lines (est.) |
|------|-------|--------------|
| `internal/infrastructure/plugins/host/progress.go` | 1.5 | ~150 |
| `pkg/plugin/sdk/progress.go` | 1.5 | ~100 |
| `plugins/recommendations/internal/user_embedding.go` | 1 | ~80 |
| `plugins/recommendations/internal/cf/types.go` | 1.5 | ~30 |
| `plugins/recommendations/internal/cf/sar.go` | 1.5 | ~150 |
| `plugins/recommendations/internal/hybrid.go` | 3 | ~100 |
| `plugins/recommendations/internal/user_embedding_test.go` | 1 | ~50 |
| `plugins/recommendations/internal/cf/sar_test.go` | 1.5 | ~100 |
| `plugins/recommendations/internal/hybrid_test.go` | 3 | ~80 |

### Files to Modify (Summary)

| File | Phase | Changes |
|------|-------|---------|
| `api/proto/plugin/host_services.proto` | 1.5 | Add HostProgress service |
| `api/proto/plugin/plugin_core.proto` | 1.5 | Add host_progress_broker_id |
| `internal/infrastructure/plugins/grpc/plugin.go` | 1.5 | Add HostProgressPlugin |
| `internal/infrastructure/plugins/grpc/factory.go` | 1.5 | Add NewHostProgressGRPCPlugin |
| `internal/infrastructure/plugins/manager/loader.go` | 1.5 | Wire HostProgress |
| `internal/app/services/services.go` | 1.5 | Create HostProgress server |
| `pkg/plugin/sdk/host.go` | 1.5 | Add Progress to HostServices |
| `plugins/recommendations/internal/recommendations.go` | 1, 1.5, 3 | User embedding, SAR, hybrid |
| `plugins/recommendations/internal/plugin.go` | 1, 1.5, 4 | Wire services, scheduling |

### Deferred: Phase 2 (BPR)

BPR will only be implemented if SAR recommendation quality is insufficient. Tasks if needed:

- [ ] `plugins/recommendations/internal/cf/bpr.go` (~200 lines)
- [ ] `plugins/recommendations/internal/cf/model.go` (~100 lines)
- [ ] `plugins/recommendations/internal/cf/trainer.go` (~100 lines)
- [ ] `plugins/recommendations/internal/cf/sampler.go` (~50 lines)
- [ ] `plugins/recommendations/internal/cf/bpr_test.go`
