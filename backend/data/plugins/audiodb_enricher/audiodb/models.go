package audiodb

// ArtistResponse represents the response from AudioDB artist search
type ArtistResponse struct {
	Artists []Artist `json:"artists"`
}

// Artist represents an artist from AudioDB
type Artist struct {
	IDArtist           string `json:"idArtist"`
	StrArtist          string `json:"strArtist"`
	StrArtistStripped  string `json:"strArtistStripped"`
	StrArtistAlternate string `json:"strArtistAlternate"`
	StrLabel           string `json:"strLabel"`
	IntFormedYear      string `json:"intFormedYear"`
	IntBornYear        string `json:"intBornYear"`
	IntDiedYear        string `json:"intDiedYear"`
	StrDisbanded       string `json:"strDisbanded"`
	StrStyle           string `json:"strStyle"`
	StrGenre           string `json:"strGenre"`
	StrMood            string `json:"strMood"`
	StrWebsite         string `json:"strWebsite"`
	StrFacebook        string `json:"strFacebook"`
	StrTwitter         string `json:"strTwitter"`
	StrBiographyEN     string `json:"strBiographyEN"`
	StrCountry         string `json:"strCountry"`
	StrArtistThumb     string `json:"strArtistThumb"`
	StrArtistLogo      string `json:"strArtistLogo"`
	StrArtistCutout    string `json:"strArtistCutout"`
	StrArtistClearart  string `json:"strArtistClearart"`
	StrArtistWideThumb string `json:"strArtistWideThumb"`
	StrArtistFanart    string `json:"strArtistFanart"`
	StrArtistFanart2   string `json:"strArtistFanart2"`
	StrArtistFanart3   string `json:"strArtistFanart3"`
	StrArtistBanner    string `json:"strArtistBanner"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrISNIcode        string `json:"strISNIcode"`
	StrLastFMChart     string `json:"strLastFMChart"`
	IntCharted         string `json:"intCharted"`
	StrLocked          string `json:"strLocked"`
}

// AlbumResponse represents the response from AudioDB album search
type AlbumResponse struct {
	Album []Album `json:"album"`
}

// Album represents an album from AudioDB
type Album struct {
	IDAlbum            string `json:"idAlbum"`
	IDArtist           string `json:"idArtist"`
	StrAlbum           string `json:"strAlbum"`
	StrAlbumStripped   string `json:"strAlbumStripped"`
	StrArtist          string `json:"strArtist"`
	StrArtistStripped  string `json:"strArtistStripped"`
	IntYearReleased    string `json:"intYearReleased"`
	StrStyle           string `json:"strStyle"`
	StrGenre           string `json:"strGenre"`
	StrLabel           string `json:"strLabel"`
	StrReleaseFormat   string `json:"strReleaseFormat"`
	IntSales           string `json:"intSales"`
	StrAlbumThumb      string `json:"strAlbumThumb"`
	StrAlbumThumbHQ    string `json:"strAlbumThumbHQ"`
	StrAlbumThumbBack  string `json:"strAlbumThumbBack"`
	StrAlbumCDart      string `json:"strAlbumCDart"`
	StrAlbumSpine      string `json:"strAlbumSpine"`
	StrAlbum3DCase     string `json:"strAlbum3DCase"`
	StrAlbum3DFlat     string `json:"strAlbum3DFlat"`
	StrAlbum3DFace     string `json:"strAlbum3DFace"`
	StrAlbum3DThumb    string `json:"strAlbum3DThumb"`
	StrDescriptionEN   string `json:"strDescriptionEN"`
	IntLoved           string `json:"intLoved"`
	IntScore           string `json:"intScore"`
	IntScoreVotes      string `json:"intScoreVotes"`
	StrReview          string `json:"strReview"`
	StrMood            string `json:"strMood"`
	StrTheme           string `json:"strTheme"`
	StrSpeed           string `json:"strSpeed"`
	StrLocation        string `json:"strLocation"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrMusicBrainzArtistID string `json:"strMusicBrainzArtistID"`
	StrAllMusicID      string `json:"strAllMusicID"`
	StrBBCReviewID     string `json:"strBBCReviewID"`
	StrRateYourMusicID string `json:"strRateYourMusicID"`
	StrDiscogsID       string `json:"strDiscogsID"`
	StrWikidataID      string `json:"strWikidataID"`
	StrWikipediaID     string `json:"strWikipediaID"`
	StrGeniusID        string `json:"strGeniusID"`
	StrLyricFind       string `json:"strLyricFind"`
	StrMusicMozID      string `json:"strMusicMozID"`
	StrItunesID        string `json:"strItunesID"`
	StrAmazonID        string `json:"strAmazonID"`
	StrLocked          string `json:"strLocked"`
}

// TrackResponse represents the response from AudioDB track search
type TrackResponse struct {
	Track []Track `json:"track"`
}

// Track represents a track from AudioDB
type Track struct {
	IDTrack            string `json:"idTrack"`
	IDArtist           string `json:"idArtist"`
	IDAlbum            string `json:"idAlbum"`
	IDIMVDB            string `json:"idIMVDB"`
	IDLyric            string `json:"idLyric"`
	StrTrack           string `json:"strTrack"`
	StrAlbum           string `json:"strAlbum"`
	StrArtist          string `json:"strArtist"`
	StrArtistAlternate string `json:"strArtistAlternate"`
	IntCD              string `json:"intCD"`
	IntTrackNumber     string `json:"intTrackNumber"`
	StrGenre           string `json:"strGenre"`
	StrMood            string `json:"strMood"`
	StrStyle           string `json:"strStyle"`
	StrTheme           string `json:"strTheme"`
	StrDescriptionEN   string `json:"strDescriptionEN"`
	StrTrackLyrics     string `json:"strTrackLyrics"`
	StrMVID            string `json:"strMVID"`
	StrTrackThumb      string `json:"strTrackThumb"`
	StrTrack3DCase     string `json:"strTrack3DCase"`
	IntLoved           string `json:"intLoved"`
	IntScore           string `json:"intScore"`
	IntScoreVotes      string `json:"intScoreVotes"`
	IntDuration        string `json:"intDuration"`
	StrLocked          string `json:"strLocked"`
	StrMusicVid        string `json:"strMusicVid"`
	StrMusicVidDirector string `json:"strMusicVidDirector"`
	StrMusicVidCompany string `json:"strMusicVidCompany"`
	StrMusicVidScreen1 string `json:"strMusicVidScreen1"`
	StrMusicVidScreen2 string `json:"strMusicVidScreen2"`
	StrMusicVidScreen3 string `json:"strMusicVidScreen3"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrMusicBrainzAlbumID string `json:"strMusicBrainzAlbumID"`
	StrMusicBrainzArtistID string `json:"strMusicBrainzArtistID"`
	StrLyricFind       string `json:"strLyricFind"`
} 