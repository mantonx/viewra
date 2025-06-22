import React from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { MediaPlayer } from '../../components/MediaPlayer';

const EpisodePlayer: React.FC = () => {
  const navigate = useNavigate();
  const { episodeId } = useParams<{ episodeId: string }>();

  const handleBack = () => {
    // Navigate back to the show detail page
    navigate(-1);
  };

  if (!episodeId) {
    return <div>Episode ID not found</div>;
  }

  // Note: In a real implementation, you would need to fetch the episode data
  // to get the tvShowId, seasonNumber, and episodeNumber.
  // For now, we'll need to parse these from the episodeId or fetch from an API.
  // This is a temporary implementation - you'll need to update this based on your data structure.
  return (
    <MediaPlayer
      type="episode"
      tvShowId={1} // TODO: Fetch actual tvShowId from episode data
      seasonNumber={1} // TODO: Fetch actual seasonNumber from episode data
      episodeNumber={1} // TODO: Fetch actual episodeNumber from episode data
      autoplay={true}
      onBack={handleBack}
    />
  );
};

export default EpisodePlayer;