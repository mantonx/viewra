import React from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { MediaPlayer } from '../../components/MediaPlayer';

const MoviePlayer: React.FC = () => {
  const navigate = useNavigate();
  const { movieId } = useParams<{ movieId: string }>();

  const handleBack = () => {
    // Navigate back to the movies page or movie detail
    navigate(-1);
  };

  if (!movieId) {
    return <div>Movie ID not found</div>;
  }

  return (
    <MediaPlayer
      type="movie"
      movieId={parseInt(movieId, 10)}
      autoplay={true}
      onBack={handleBack}
    />
  );
};

export default MoviePlayer;