# Resource Management Testing Plan

## Setup

1. **Open a new terminal** and run the monitor script:
   ```bash
   ./scripts/monitor-ffmpeg.sh
   ```
   This will continuously show you active and zombie FFmpeg processes.

2. **Keep your browser open** to http://localhost:5173

## Test 1: Basic Playback (Baseline)

**Goal**: Verify 1 process during playback, 0 when idle

**Steps**:
1. ✅ Confirm 0 FFmpeg processes initially
2. ✅ Play any video in the browser
3. ✅ Check monitor: Should show **1 Active, 0 Zombie**
4. ✅ Stop/pause the video
5. ✅ Wait 35-40 seconds
6. ✅ Check monitor: Should show **0 Active, 0 Zombie**

**Expected Result**: ✅ Process appears during playback, disappears after 30s idle

---

## Test 2: Seeking Within Video (Session Reuse)

**Goal**: Verify no zombie processes when seeking, session is reused

**Steps**:
1. ✅ Play a video
2. ✅ Check monitor: **1 Active, 0 Zombie**
3. ✅ Skip ahead 10 seconds (small seek)
4. ✅ Check monitor: Should still be **1 Active, 0 Zombie** (same session)
5. ✅ Skip ahead 20 seconds
6. ✅ Check monitor: Should still be **1 Active, 0 Zombie**
7. ✅ Skip ahead 30 seconds
8. ✅ Check monitor: Should still be **1 Active, 0 Zombie**
9. ✅ Stop video and wait 40 seconds
10. ✅ Check monitor: Should be **0 Active, 0 Zombie**

**Expected Result**: ✅ Always 1 process during playback, no zombies accumulate

---

## Test 3: Video Switching (Immediate Cleanup)

**Goal**: Verify old session stops immediately when switching videos

**Steps**:
1. ✅ Play Video A
2. ✅ Check monitor: **1 Active, 0 Zombie**
3. ✅ Navigate to and play Video B (different movie/episode)
4. ✅ Check monitor immediately: Should show **1 Active, 0 Zombie** (new session)
5. ✅ Check backend logs for "Stopping session for different media"
6. ✅ Stop Video B and wait 40 seconds
7. ✅ Check monitor: **0 Active, 0 Zombie**

**Expected Result**: ✅ Immediate switch, no accumulation, no zombies

---

## Test 4: Rapid Video Switching (Stress Test)

**Goal**: Ensure no zombie accumulation under rapid switching

**Steps**:
1. ✅ Play Video A for 5 seconds
2. ✅ Switch to Video B for 5 seconds
3. ✅ Switch to Video C for 5 seconds
4. ✅ Switch to Video D for 5 seconds
5. ✅ Check monitor throughout: Should always be **1 Active, 0 Zombie**
6. ✅ Stop playback and wait 40 seconds
7. ✅ Check monitor: **0 Active, 0 Zombie**

**Expected Result**: ✅ No zombie processes, clean transitions

---

## Test 5: Backend Logs Check

**Goal**: Verify cleanup messages appear in logs

**In a separate terminal, run**:
```bash
# Filter for cleanup-related log messages
tail -f .overmind.sock | grep -E "(Stopping|Cleaning|Reusing|Created new)"
```

**Or check the overmind output**:
```bash
# In the overmind terminal, look for messages like:
# - "Stopping session for different media"
# - "Reusing existing session"
# - "Created new transcode session"
# - "Cleaned up idle sessions"
```

**Expected Result**: ✅ Appropriate log messages for each action

---

## Success Criteria

All tests should show:
- ✅ **Never more than 1 Active FFmpeg process** during single video playback
- ✅ **Always 0 Zombie processes** (no `<defunct>`)
- ✅ **Automatic cleanup after 30-40 seconds** of idle time
- ✅ **Immediate cleanup when switching videos**
- ✅ **Appropriate log messages** for each cleanup action

## Failure Indicators

❌ Multiple active processes for single video
❌ Zombie `<defunct>` processes appearing
❌ Processes not cleaning up after 40+ seconds
❌ No log messages about stopping/cleaning sessions

---

## How to Read the Monitor

```
Active FFmpeg Processes: 1    <- Should be 0 or 1
Zombie FFmpeg Processes: 0    <- Should ALWAYS be 0
```

If you see zombies:
```
fiction+  12345  0.0  0.0      0     0 pts/6    Z+   11:50   0:43 [ffmpeg] <defunct>
                                               ^^                          ^^^^^^^^^^
                                            ZOMBIE                      DEFUNCT = BAD
```

Clean process:
```
fiction+  12345  160  2.6  15695888 876304 pts/6 Sl+  11:51   1:50 ffmpeg -hwaccel cuda ...
                                                   ^^
                                                 GOOD (Sleeping/Running)
```
