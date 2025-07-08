import React, { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { MediaPlayer } from '../../components/MediaPlayer';
import { API_ENDPOINTS, buildApiUrl, buildApiUrlWithParams } from '../../constants/api';

interface MediaFile {
  id: string;
  media_id: string;
  media_type: string;
  path: string;
}

interface Episode {
  episode_id: string;
  title: string;
  episode_number: number;
  season: {
    id: string;
    season_number: number;
    tv_show: {
      id: string;
      title: string;
    };
  };
}

interface EpisodeMetadata {
  metadata: {
    type: 'episode';
    episode_id: string;
    title: string;
    episode_number: number;
    season: {
      id: string;
      season_number: number;
      tv_show: {
        id: string;
        title: string;
      };
    };
  };
}

const EpisodePlayer: React.FC = () => {
  const navigate = useNavigate();
  const { episodeId } = useParams<{ episodeId: string }>();
  const [episode, setEpisode] = useState<Episode | null>(null);
  const [mediaFileId, setMediaFileId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchEpisode = async () => {
      if (!episodeId) return;

      try {
        setLoading(true);
        
        // Try to get the file directly by file ID (for MediaPlayerTest)
        const fileResponse = await fetch(buildApiUrl(API_ENDPOINTS.MEDIA.FILE_BY_ID.path(episodeId)));
        
        if (fileResponse.ok) {
          const mediaFile = await fileResponse.json();
          
          if (mediaFile.media_type === 'episode') {
            console.log('✅ Found episode file:', mediaFile.path);
            setMediaFileId(mediaFile.id);
            
            // Create episode metadata from file path for MediaPlayerTest
            const pathMatch = mediaFile.path.match(/\/tv\/([^\/]+)\/Season\s+(\d+)\/.*?S(\d+)E(\d+)/i);
            if (pathMatch) {
              const [, showName, seasonNum, , episodeNum] = pathMatch;
              
              setEpisode({
                episode_id: mediaFile.media_id || mediaFile.id,
                title: `Episode ${episodeNum}`,
                episode_number: parseInt(episodeNum, 10),
                season: {
                  id: `season-${seasonNum}`,
                  season_number: parseInt(seasonNum, 10),
                  tv_show: {
                    id: `show-${showName}`,
                    title: showName.replace(/[._-]/g, ' '),
                  },
                },
              });
            } else {
              // Fallback episode info
              setEpisode({
                episode_id: mediaFile.media_id || mediaFile.id,
                title: 'Unknown Episode',
                episode_number: 1,
                season: {
                  id: 'unknown-season',
                  season_number: 1,
                  tv_show: {
                    id: 'unknown-show',
                    title: 'Unknown Show',
                  },
                },
              });
            }
          } else {
            throw new Error('File is not an episode');
          }
        } else {
          throw new Error('Episode not found in media library');
        }
      } catch (err) {
        console.error('Failed to fetch episode:', err);
        setError(err instanceof Error ? err.message : 'Failed to load episode');
      } finally {
        setLoading(false);
      }
    };

    fetchEpisode();
  }, [episodeId]);

  const handleBack = () => {
    // Navigate back to the show detail page
    navigate(-1);
  };

  if (!episodeId) {
    return <div>Episode ID not found</div>;
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-black text-white">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-white mx-auto mb-4"></div>
          <p>Loading episode...</p>
        </div>
      </div>
    );
  }

  if (error || !episode || !mediaFileId) {
    return (
      <div className="flex items-center justify-center h-screen bg-black text-white">
        <div className="text-center max-w-md">
          <h2 className="text-xl font-bold mb-4">Error</h2>
          <p className="text-red-400 mb-4">{error || 'Episode not found'}</p>
          <button
            onClick={() => navigate(-1)}
            className="bg-gray-700 hover:bg-gray-600 px-4 py-2 rounded transition-colors"
          >
            Go Back
          </button>
        </div>
      </div>
    );
  }

  console.log('🎬 Rendering MediaPlayer with:', {
    type: 'episode',
    mediaFileId,
    episode: episode.title
  });

  return (
    <MediaPlayer
      type="episode"
      tvShowId={1} // Use dummy ID for MediaPlayerTest files
      seasonNumber={episode.season.season_number}
      episodeNumber={episode.episode_number}
      autoplay={true}
      onBack={handleBack}
    />
  );
};

export default EpisodePlayer;