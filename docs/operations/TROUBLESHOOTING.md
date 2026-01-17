# Troubleshooting Guide

Common issues and solutions for ViewRA.

---

## Startup Issues

### Server won't start

**Symptoms**: Server exits immediately or shows error on startup.

**Check**:

1. **Port in use**
   ```bash
   lsof -i :8080
   # Kill existing process or change PORT
   ```

2. **Database permissions**
   ```bash
   ls -la data/viewra.db
   # Ensure write permissions
   ```

3. **Missing FFmpeg**
   ```bash
   ffmpeg -version
   # Install FFmpeg if missing
   ```

4. **Invalid config**
   - Check `data/config.yaml` syntax
   - Validate environment variables

### JWT_SECRET required error

**In production**, you must set a JWT secret:

```bash
export JWT_SECRET="your-secure-random-string"
```

Generate a secure secret:

```bash
openssl rand -base64 32
```

---

## Playback Issues

### Video won't play / buffering

**Check**:

1. **FFmpeg availability**
   ```bash
   curl http://localhost:8080/health
   # Look for ffmpeg_available: true
   ```

2. **Transcode directory permissions**
   ```bash
   ls -la data/transcodes/
   # Ensure write permissions
   ```

3. **Disk space**
   ```bash
   df -h data/
   # Need space for transcode output
   ```

4. **Check transcode logs**
   ```bash
   # Enable debug logging
   LOG_LEVEL=debug ./viewra
   # Look for FFmpeg errors
   ```

### Hardware acceleration not working

**NVIDIA NVENC**:

```bash
# Check NVIDIA driver
nvidia-smi

# Check FFmpeg NVENC support
ffmpeg -encoders | grep nvenc
```

**Intel QuickSync/VAAPI**:

```bash
# Check render device
ls -la /dev/dri/renderD128

# Check FFmpeg support
ffmpeg -encoders | grep qsv
ffmpeg -encoders | grep vaapi
```

**Force software encoding** if hardware fails:

```bash
export VIEWRA_HW_ACCEL=none
```

### Audio out of sync

Usually caused by container format issues. Try:

1. **Check source file**
   ```bash
   ffprobe -v quiet -show_streams /path/to/file.mkv
   ```

2. **Report the issue** with the ffprobe output

---

## Library Scanning Issues

### Scan hangs or takes too long

**Check**:

1. **Network storage latency**
   - NFS/SMB mounts can be slow
   - Consider local caching

2. **Permission errors**
   ```bash
   # Check scan warnings in UI or logs
   # Look for permission denied errors
   ```

3. **Too many small files**
   - Large libraries with millions of files take time
   - Check `SCAN_PROGRESS_INTERVAL` to monitor progress

### Media not appearing after scan

**Check**:

1. **File format supported**
   - ViewRA supports: `.mkv`, `.mp4`, `.avi`, `.m4v`, `.mov`, `.wmv`, `.webm`
   - Audio: `.mp3`, `.flac`, `.m4a`, `.ogg`, `.opus`, `.wav`, `.aac`

2. **File naming**
   - Movies: `Movie Name (Year).ext` or in folder `Movie Name (Year)/`
   - TV: `Show Name/Season XX/Show Name SXXEXX.ext`

3. **Database refresh**
   - Try rescanning the library
   - Check for errors in scan job history

---

## Database Issues

### SQLite locked

**Symptoms**: "database is locked" errors

**Solutions**:

1. **Single writer** - SQLite allows only one writer at a time
2. **Check for stuck processes**
   ```bash
   lsof data/viewra.db
   ```
3. **Increase busy timeout** (in code, not configurable yet)

### PostgreSQL connection refused

**Check**:

1. **Host/port correct**
   ```bash
   psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME
   ```

2. **pg_hba.conf** allows connections from your host

3. **SSL mode** - Try `DB_SSL_MODE=disable` for testing

### Migration failed

```bash
# Check migration status
~/go/bin/migrate -database "sqlite3://./data/viewra.db" -path ./migrations version

# Force to specific version if needed
~/go/bin/migrate -database "sqlite3://./data/viewra.db" -path ./migrations force VERSION
```

---

## Plugin Issues

### Plugins not loading

**Check**:

1. **Plugins enabled**
   ```bash
   echo $PLUGINS_ENABLED  # should be true or unset
   ```

2. **Plugin binary exists and is executable**
   ```bash
   ls -la data/plugins/
   chmod +x data/plugins/*
   ```

3. **Plugin manifest exists**
   ```bash
   cat data/plugins/tmdb/plugin.yml
   ```

4. **Check logs** for plugin initialization errors

### TMDb/MusicBrainz not enriching

**Check**:

1. **API key configured** (for TMDb)
   - Go to Settings → Plugins → TMDb
   - Enter your API key

2. **Rate limiting**
   - External APIs have rate limits
   - Check logs for 429 errors

3. **Media identified correctly**
   - Check if IMDB/TMDB/TVDB IDs are present
   - NFO files help with identification

---

## Performance Issues

### High CPU usage

**Possible causes**:

1. **Transcoding** - Normal during playback
2. **Library scan** - Normal during scan
3. **Image processing** - Normal during enrichment

**Reduce load**:

- Reduce `TRANSCODE_WORKERS`
- Enable hardware acceleration
- Reduce `SCAN_PARALLEL_WALKERS`

### High memory usage

**Check**:

1. **Goroutine leak**
   ```bash
   curl http://localhost:8080/health
   # Check goroutine count
   ```

2. **Large library** - More media = more memory for caching

3. **Plugin memory** - Some plugins (semantic-search) use significant memory

### Slow API responses

**Check**:

1. **Database queries**
   - Enable debug logging
   - Look for slow query warnings

2. **Connection pool exhausted**
   - Increase `DB_MAX_OPEN_CONNS`

---

## Logs

### Enable debug logging

```bash
LOG_LEVEL=debug ./viewra
```

### Key log locations

- Startup: Look for `Starting ViewRA server`
- FFmpeg: Look for `ffmpeg` or `transcode`
- Scan: Look for `scan` or `library`
- Plugins: Look for `plugin`

### Common log patterns

```text
# Successful startup
INFO Starting ViewRA server port=8080

# Database connected
INFO Database connection established driver=sqlite

# Plugin loaded
INFO Plugin loaded name=tmdb version=1.0.0

# Transcode started
INFO Starting transcode session media_id=123 quality=1080p-10m

# Scan completed
INFO Library scan completed library_id=1 files_found=1000 duration=5m30s
```

---

## Getting Help

1. **Check logs** with `LOG_LEVEL=debug`
2. **Check health endpoint** at `/health`
3. **Search existing issues** on GitHub
4. **Report issues** at https://github.com/viewra/viewra/issues with:
   - ViewRA version
   - Operating system
   - Relevant logs (sanitize secrets)
   - Steps to reproduce
