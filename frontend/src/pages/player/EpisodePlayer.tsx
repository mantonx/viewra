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
        
        // First try to get the file directly by file ID
        let mediaFile = null;
        const fileResponse = await fetch(buildApiUrl(API_ENDPOINTS.MEDIA.FILE_BY_ID.path(episodeId)));
        
        if (fileResponse.ok) {
          const fileData = await fileResponse.json();
          mediaFile = fileData.media_file;
        } else if (fileResponse.status === 404) {
          // If not found by file ID, search by media ID (episode metadata ID)
          console.log('🔍 File ID not found, searching by media ID...');
          const searchUrl = buildApiUrlWithParams('/media/', { limit: 50000 });
          console.log('🎯 Search URL:', searchUrl);
          
          const searchResponse = await fetch(searchUrl);
          console.log('📡 Search response status:', searchResponse.status);
          
          if (searchResponse.ok) {
            const searchData = await searchResponse.json();
            console.log('📁 Total media files:', searchData.media?.length || 0);
            
            const foundFile = searchData.media?.find(
              (file: any) => file.media_id === episodeId && file.media_type === 'episode'
            );
            
            if (foundFile) {
              console.log('✅ Found episode by media ID, file ID:', foundFile.id);
              mediaFile = foundFile;
            } else {
              console.log('❌ Episode not found in search results');
              console.log('🔍 Looking for media_id:', episodeId);
            }
          } else {
            console.error('❌ Search failed:', searchResponse.statusText);
          }
        }
        
        if (!mediaFile || mediaFile.media_type !== 'episode') {
          throw new Error('Episode not found in media library');
        }
        
        console.log('✅ Found episode:', mediaFile.path);
        setMediaFileId(mediaFile.id);
        
        // Now get the metadata for this file
        const metadataResponse = await fetch(buildApiUrl(API_ENDPOINTS.MEDIA.FILE_METADATA.path(mediaFile.id)));
        if (!metadataResponse.ok) {
          throw new Error('Failed to fetch episode metadata');
        }
        
        const metadataData: EpisodeMetadata = await metadataResponse.json();
        
        if (metadataData.metadata?.type === 'episode') {
          setEpisode({
            episode_id: metadataData.metadata.episode_id,
            title: metadataData.metadata.title,
            episode_number: metadataData.metadata.episode_number,
            season: metadataData.metadata.season,
          });
        } else {
          throw new Error('Invalid metadata type');
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
    tvShowId: parseInt(episode.season.tv_show.id),
    seasonNumber: episode.season.season_number,
    episodeNumber: episode.episode_number,
    mediaFileId
  });

  return (
    <MediaPlayer
      type="episode"
      tvShowId={parseInt(episode.season.tv_show.id)}
      seasonNumber={episode.season.season_number}
      episodeNumber={episode.episode_number}
      autoplay={true}
      onBack={handleBack}
    />
  );
};

export default EpisodePlayer;