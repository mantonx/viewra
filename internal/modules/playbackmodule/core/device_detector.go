// Package core provides the core functionality for the playback module.
package core

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// DeviceCapabilities represents what a device/client can play
type DeviceCapabilities struct {
	// Video codec support
	H264 bool `json:"h264"`
	H265 bool `json:"h265"`
	VP8  bool `json:"vp8"`
	VP9  bool `json:"vp9"`
	AV1  bool `json:"av1"`

	// Audio codec support
	AAC    bool `json:"aac"`
	MP3    bool `json:"mp3"`
	Opus   bool `json:"opus"`
	Vorbis bool `json:"vorbis"`
	FLAC   bool `json:"flac"`
	ALAC   bool `json:"alac"`   // Apple Lossless
	PCM    bool `json:"pcm"`    // WAV/AIFF
	DTS    bool `json:"dts"`    // DTS audio
	AC3    bool `json:"ac3"`    // Dolby Digital
	EAC3   bool `json:"eac3"`   // Dolby Digital Plus
	TrueHD bool `json:"truehd"` // Dolby TrueHD
	DTSHD  bool `json:"dtshd"`  // DTS-HD

	// Container support
	MP4           bool `json:"mp4"`
	WebM          bool `json:"webm"`
	MKV           bool `json:"mkv"`
	OGG           bool `json:"ogg"`            // OGG container
	M4A           bool `json:"m4a"`            // Audio-only MP4
	WAV           bool `json:"wav"`            // WAV audio
	FLACContainer bool `json:"flac_container"` // FLAC container

	// Feature support
	MSE    bool `json:"mse"` // Media Source Extensions
	EME    bool `json:"eme"` // Encrypted Media Extensions
	WebRTC bool `json:"webrtc"`
	HLS    bool `json:"hls"` // Native HLS support (Safari)

	// Performance hints
	MaxResolution string `json:"max_resolution"`
	Hardware      bool   `json:"hardware"` // Hardware acceleration available
}

// DeviceDetector analyzes user agents and client identifiers to determine capabilities
type DeviceDetector struct {
	logger hclog.Logger
}

// NewDeviceDetector creates a new device detector
func NewDeviceDetector(logger hclog.Logger) *DeviceDetector {
	return &DeviceDetector{
		logger: logger,
	}
}

// DetectCapabilities analyzes a user agent string to determine device capabilities
func (dd *DeviceDetector) DetectCapabilities(userAgent string) *DeviceCapabilities {
	ua := strings.ToLower(userAgent)

	// Default capabilities - most modern browsers support these
	caps := &DeviceCapabilities{
		H264:          true, // Universally supported
		AAC:           true, // Universally supported
		MP3:           true, // Universally supported
		MP4:           true, // Universally supported
		MSE:           true, // Most modern browsers
		WAV:           true, // Universally supported
		PCM:           true, // WAV/AIFF support
		M4A:           true, // Audio MP4 support
		MaxResolution: "1080p",
	}

	// First check for native apps and devices
	switch {
	// Apple devices
	case strings.Contains(ua, "viewra/ios") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		dd.detectiOS(ua, caps)

	case strings.Contains(ua, "viewra/tvos") || strings.Contains(ua, "appletv"):
		dd.detectAppleTV(ua, caps)

	// Android devices
	case strings.Contains(ua, "viewra/android") || (strings.Contains(ua, "android") && !strings.Contains(ua, "chrome")):
		dd.detectAndroidApp(ua, caps)

	case strings.Contains(ua, "nvidia shield") || strings.Contains(ua, "shield android tv"):
		dd.detectNvidiaShield(ua, caps)

	// Roku
	case strings.Contains(ua, "roku"):
		dd.detectRoku(ua, caps)

	// Smart TVs
	case strings.Contains(ua, "smarttv") || strings.Contains(ua, "webos") || strings.Contains(ua, "tizen"):
		dd.detectSmartTV(ua, caps)

	// Web browsers
	case strings.Contains(ua, "chrome") || strings.Contains(ua, "chromium"):
		dd.detectChrome(ua, caps)

	case strings.Contains(ua, "firefox"):
		dd.detectFirefox(ua, caps)

	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		dd.detectSafari(ua, caps)

	case strings.Contains(ua, "edge"):
		dd.detectEdge(ua, caps)

	case strings.Contains(ua, "opera"):
		dd.detectOpera(ua, caps)
	}

	// Detect mobile/tablet
	if dd.isMobile(ua) {
		caps.MaxResolution = "720p" // Conservative for mobile
	}

	// Detect 4K support based on platform
	if dd.supports4K(ua) {
		caps.MaxResolution = "2160p"
	}

	dd.logger.Debug("Detected device capabilities",
		"userAgent", userAgent,
		"h264", caps.H264,
		"h265", caps.H265,
		"vp9", caps.VP9,
		"webm", caps.WebM,
		"maxResolution", caps.MaxResolution)

	return caps
}

// detectiOS sets capabilities for iOS devices (iPhone/iPad)
func (dd *DeviceDetector) detectiOS(ua string, caps *DeviceCapabilities) {
	caps.H265 = true // iOS supports HEVC
	caps.HLS = true  // Native HLS support
	caps.Hardware = true
	caps.MSE = false  // No MSE in iOS Safari
	caps.WebM = false // No WebM support
	caps.VP8 = false
	caps.VP9 = false
	caps.AV1 = false
	caps.Opus = false
	caps.Vorbis = false

	// iOS audio support
	caps.ALAC = true // Apple Lossless
	caps.FLAC = true // iOS 11+ supports FLAC
	caps.AC3 = true  // Dolby Digital
	caps.EAC3 = true // Dolby Digital Plus

	// iOS 14+ supports 4K
	if strings.Contains(ua, "os 14") || strings.Contains(ua, "os 15") || strings.Contains(ua, "os 16") || strings.Contains(ua, "os 17") {
		caps.MaxResolution = "2160p"
	}
}

// detectAppleTV sets capabilities for Apple TV
func (dd *DeviceDetector) detectAppleTV(ua string, caps *DeviceCapabilities) {
	caps.H265 = true // Apple TV supports HEVC
	caps.HLS = true  // Native HLS support
	caps.Hardware = true
	caps.MaxResolution = "2160p" // Apple TV 4K
	caps.MSE = false
	caps.WebM = false
	caps.VP8 = false
	caps.VP9 = false
	caps.AV1 = false
	caps.Opus = false
	caps.Vorbis = false
	caps.EME = true // DRM support

	// Apple TV audio support
	caps.ALAC = true   // Apple Lossless
	caps.FLAC = true   // FLAC support
	caps.AC3 = true    // Dolby Digital
	caps.EAC3 = true   // Dolby Digital Plus
	caps.TrueHD = true // Dolby TrueHD
	caps.DTS = true    // DTS
	caps.DTSHD = true  // DTS-HD
}

// detectAndroidApp sets capabilities for Android native app
func (dd *DeviceDetector) detectAndroidApp(ua string, caps *DeviceCapabilities) {
	caps.H265 = true // Most modern Android devices support HEVC
	caps.VP9 = true  // Good VP9 support
	caps.Hardware = true
	caps.WebM = true
	caps.Opus = true
	caps.HLS = false // No native HLS
	caps.MSE = false // Native app doesn't use MSE

	// Android audio support
	caps.FLAC = true // Android supports FLAC
	caps.OGG = true  // OGG Vorbis support
	caps.AC3 = true  // Some devices support AC3
	caps.EAC3 = true // Some devices support E-AC3

	// Check Android version for capabilities
	if strings.Contains(ua, "android 10") || strings.Contains(ua, "android 11") || strings.Contains(ua, "android 12") || strings.Contains(ua, "android 13") || strings.Contains(ua, "android 14") {
		caps.AV1 = true
	}
}

// detectNvidiaShield sets capabilities for Nvidia Shield
func (dd *DeviceDetector) detectNvidiaShield(ua string, caps *DeviceCapabilities) {
	caps.H265 = true // Excellent HEVC support
	caps.VP9 = true
	caps.AV1 = true // Shield supports AV1
	caps.Hardware = true
	caps.MaxResolution = "2160p"
	caps.WebM = true
	caps.Opus = true
	caps.EME = true // DRM support
	caps.HLS = false
	caps.MSE = false

	// Shield audio support - excellent codec support
	caps.FLAC = true
	caps.ALAC = true
	caps.DTS = true
	caps.DTSHD = true
	caps.AC3 = true
	caps.EAC3 = true
	caps.TrueHD = true
}

// detectRoku sets capabilities for Roku devices
func (dd *DeviceDetector) detectRoku(ua string, caps *DeviceCapabilities) {
	caps.H265 = true // Roku 4K devices support HEVC
	caps.Hardware = true
	caps.HLS = true // Good HLS support
	caps.MSE = false
	caps.WebM = false // Limited WebM support
	caps.VP8 = false
	caps.VP9 = false
	caps.AV1 = false
	caps.Opus = false
	caps.Vorbis = false

	// Roku 4K models
	if strings.Contains(ua, "4800") || strings.Contains(ua, "4802") || strings.Contains(ua, "4670") {
		caps.MaxResolution = "2160p"
	}
}

// detectSmartTV sets capabilities for generic Smart TVs
func (dd *DeviceDetector) detectSmartTV(ua string, caps *DeviceCapabilities) {
	caps.H265 = true // Most smart TVs support HEVC
	caps.Hardware = true
	caps.MaxResolution = "2160p" // Assume 4K TV
	caps.HLS = true
	caps.MSE = false
	caps.WebM = false // Limited WebM on TVs
	caps.VP8 = false
	caps.VP9 = false
	caps.AV1 = false // Few TVs support AV1 yet
	caps.Opus = false
	caps.Vorbis = false
	caps.EME = true // DRM support
}

// detectChrome sets capabilities for Chrome/Chromium browsers
func (dd *DeviceDetector) detectChrome(ua string, caps *DeviceCapabilities) {
	caps.VP8 = true
	caps.VP9 = true
	caps.WebM = true
	caps.Opus = true
	caps.Vorbis = true
	caps.WebRTC = true

	// Firefox audio support
	caps.FLAC = true // Firefox supports FLAC
	caps.OGG = true  // Excellent OGG support
	caps.EME = true

	// Chrome audio support
	caps.FLAC = true // Chrome supports FLAC
	caps.OGG = true  // OGG container support

	// Chrome version checks
	if dd.getChromeVersion(ua) >= 90 {
		caps.AV1 = true
	}

	// Chrome on Windows might support HEVC
	if strings.Contains(ua, "windows") {
		caps.H265 = dd.checkWindowsHEVC(ua)
	}

	caps.Hardware = true // Chrome generally has good hardware acceleration
}

// detectFirefox sets capabilities for Firefox
func (dd *DeviceDetector) detectFirefox(ua string, caps *DeviceCapabilities) {
	caps.VP8 = true
	caps.VP9 = true
	caps.WebM = true
	caps.Opus = true
	caps.Vorbis = true
	caps.WebRTC = true

	// Firefox version checks
	version := dd.getFirefoxVersion(ua)
	if version >= 100 {
		caps.AV1 = true
	}

	// Firefox doesn't support H.265/HEVC
	caps.H265 = false

	// Firefox EME support varies
	caps.EME = version >= 47
}

// detectSafari sets capabilities for Safari
func (dd *DeviceDetector) detectSafari(ua string, caps *DeviceCapabilities) {
	caps.HLS = true // Native HLS support
	caps.EME = true

	// Safari on macOS/iOS supports HEVC
	if strings.Contains(ua, "mac") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		caps.H265 = true
		caps.Hardware = true
	}

	// Safari has limited WebM support
	caps.WebM = false
	caps.VP8 = false
	caps.VP9 = false
	caps.Opus = false
	caps.Vorbis = false

	// No AV1 support in Safari
	caps.AV1 = false

	// Safari audio support
	caps.ALAC = true // Apple Lossless
	caps.FLAC = true // Safari 14+ supports FLAC
	caps.AC3 = true  // Dolby Digital
	caps.EAC3 = true // Dolby Digital Plus
}

// detectEdge sets capabilities for Microsoft Edge
func (dd *DeviceDetector) detectEdge(ua string, caps *DeviceCapabilities) {
	// Modern Edge is Chromium-based
	if strings.Contains(ua, "edg/") {
		dd.detectChrome(ua, caps)

		// Edge on Windows supports HEVC
		if strings.Contains(ua, "windows") {
			caps.H265 = true
		}
	} else {
		// Legacy Edge
		caps.WebM = false
		caps.VP8 = false
		caps.VP9 = false
		caps.EME = true
	}
}

// detectOpera sets capabilities for Opera
func (dd *DeviceDetector) detectOpera(ua string, caps *DeviceCapabilities) {
	// Opera is Chromium-based
	dd.detectChrome(ua, caps)
}

// isMobile detects if the user agent is a mobile device
func (dd *DeviceDetector) isMobile(ua string) bool {
	mobileKeywords := []string{
		"mobile", "android", "iphone", "ipad", "ipod",
		"blackberry", "windows phone", "webos",
	}

	for _, keyword := range mobileKeywords {
		if strings.Contains(ua, keyword) {
			return true
		}
	}

	return false
}

// supports4K checks if the platform likely supports 4K video
func (dd *DeviceDetector) supports4K(ua string) bool {
	// Desktop browsers on modern OS
	if strings.Contains(ua, "windows nt 10") ||
		strings.Contains(ua, "windows nt 11") ||
		strings.Contains(ua, "mac os x 10_15") ||
		strings.Contains(ua, "mac os x 11") ||
		strings.Contains(ua, "mac os x 12") {
		return !dd.isMobile(ua)
	}

	// Some high-end mobile devices
	if strings.Contains(ua, "iphone13") ||
		strings.Contains(ua, "iphone14") ||
		strings.Contains(ua, "iphone15") ||
		strings.Contains(ua, "ipad pro") {
		return true
	}

	return false
}

// getChromeVersion extracts Chrome version from user agent
func (dd *DeviceDetector) getChromeVersion(ua string) int {
	// Simple version extraction - returns major version
	parts := strings.Split(ua, "chrome/")
	if len(parts) > 1 {
		versionStr := strings.Split(parts[1], ".")[0]
		version := 0
		fmt.Sscanf(versionStr, "%d", &version)
		return version
	}
	return 0
}

// getFirefoxVersion extracts Firefox version from user agent
func (dd *DeviceDetector) getFirefoxVersion(ua string) int {
	parts := strings.Split(ua, "firefox/")
	if len(parts) > 1 {
		versionStr := strings.Split(parts[1], ".")[0]
		version := 0
		fmt.Sscanf(versionStr, "%d", &version)
		return version
	}
	return 0
}

// checkWindowsHEVC checks if Windows has HEVC support
func (dd *DeviceDetector) checkWindowsHEVC(ua string) bool {
	// Windows 10/11 might have HEVC support if codecs are installed
	// This is a heuristic - actual support depends on installed codecs
	return strings.Contains(ua, "windows nt 10") || strings.Contains(ua, "windows nt 11")
}

// ConvertToDeviceProfile converts device capabilities to codec lists for DeviceProfile
func (dd *DeviceDetector) ConvertToDeviceProfile(caps *DeviceCapabilities) (videoCodecs []string, audioCodecs []string, containers []string) {
	// Video codecs
	if caps.H264 {
		videoCodecs = append(videoCodecs, "h264", "avc")
	}
	if caps.H265 {
		videoCodecs = append(videoCodecs, "h265", "hevc")
	}
	if caps.VP8 {
		videoCodecs = append(videoCodecs, "vp8")
	}
	if caps.VP9 {
		videoCodecs = append(videoCodecs, "vp9")
	}
	if caps.AV1 {
		videoCodecs = append(videoCodecs, "av1")
	}

	// Audio codecs
	if caps.AAC {
		audioCodecs = append(audioCodecs, "aac")
	}
	if caps.MP3 {
		audioCodecs = append(audioCodecs, "mp3")
	}
	if caps.Opus {
		audioCodecs = append(audioCodecs, "opus")
	}
	if caps.Vorbis {
		audioCodecs = append(audioCodecs, "vorbis")
	}
	if caps.FLAC {
		audioCodecs = append(audioCodecs, "flac")
	}
	if caps.ALAC {
		audioCodecs = append(audioCodecs, "alac")
	}
	if caps.PCM {
		audioCodecs = append(audioCodecs, "pcm", "pcm_s16le", "pcm_s24le", "pcm_s32le")
	}
	if caps.AC3 {
		audioCodecs = append(audioCodecs, "ac3")
	}
	if caps.EAC3 {
		audioCodecs = append(audioCodecs, "eac3")
	}
	if caps.DTS {
		audioCodecs = append(audioCodecs, "dts")
	}
	if caps.DTSHD {
		audioCodecs = append(audioCodecs, "dts-hd")
	}
	if caps.TrueHD {
		audioCodecs = append(audioCodecs, "truehd")
	}

	// Containers
	if caps.MP4 {
		containers = append(containers, "mp4", "m4v")
	}
	if caps.M4A {
		containers = append(containers, "m4a")
	}
	if caps.WebM {
		containers = append(containers, "webm")
	}
	if caps.MKV {
		containers = append(containers, "mkv")
	}
	if caps.OGG {
		containers = append(containers, "ogg", "oga", "ogv")
	}
	if caps.WAV {
		containers = append(containers, "wav")
	}
	if caps.FLACContainer {
		containers = append(containers, "flac")
	}

	return videoCodecs, audioCodecs, containers
}
