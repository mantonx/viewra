import React, { useState } from 'react';
import { createPortal } from 'react-dom';
import { cn } from '@/lib/utils';
import { Tooltip } from 'react-tooltip';
import ImageModal from '@/components/ui/ImageModal';
import { Music } from '@/components/ui/icons';

interface AlbumArtworkProps {
  artworkUrl?: string;
  altText?: string;
  isPlaying?: boolean;
  isMinimized?: boolean;
  onExpandPlayer?: () => void;
  className?: string;
  showExpandModal?: boolean;
  // Additional track info for modal
  trackTitle?: string;
  artistName?: string;
  albumName?: string;
}

const AlbumArtwork: React.FC<AlbumArtworkProps> = ({
  artworkUrl,
  altText = 'Album Artwork',
  isPlaying = false,
  isMinimized = false,
  onExpandPlayer,
  className = 'w-16 h-16',
  showExpandModal = true,
  trackTitle,
  artistName,
  albumName,
}) => {
  const [showModal, setShowModal] = useState(false);
  const [imageError, setImageError] = useState(false);
  const [imageLoaded, setImageLoaded] = useState(false);

  const handleClick = () => {
    if (showExpandModal && artworkUrl) {
      setShowModal(true);
    } else if (onExpandPlayer) {
      onExpandPlayer();
    }
  };

  const handleImageError = () => {
    setImageError(true);
    setImageLoaded(false);
  };

  const handleImageLoad = () => {
    setImageError(false);
    setImageLoaded(true);
  };

  // Show fallback if no artwork URL or image failed to load
  const showFallback = !artworkUrl || imageError;

  const artworkElement = (
    <div className="relative w-full h-full group">
      {showFallback ? (
        // Fallback artwork with music icon
        <div className="w-full h-full flex items-center justify-center bg-gradient-to-br from-slate-600 to-slate-700">
          <Music size={isMinimized ? 16 : 24} className="text-slate-400" />
        </div>
      ) : (
        // Actual artwork image
        <img
          src={artworkUrl}
          alt={altText}
          className={cn(
            'w-full h-full object-cover transition-opacity duration-300',
            imageLoaded ? 'opacity-100' : 'opacity-0'
          )}
          onError={handleImageError}
          onLoad={handleImageLoad}
          loading="lazy"
        />
      )}

      {/* Loading state */}
      {artworkUrl && !imageLoaded && !imageError && (
        <div className="absolute inset-0 flex items-center justify-center bg-slate-700 animate-pulse">
          <Music size={isMinimized ? 16 : 24} className="text-slate-500" />
        </div>
      )}

      {/* Playing indicator */}
      {isPlaying && !isMinimized && (
        <div className="absolute inset-0 bg-black/20 flex items-center justify-center">
          <div className="w-8 h-8 bg-white/90 rounded-full flex items-center justify-center">
            <div className="w-0 h-0 border-l-[6px] border-l-slate-800 border-y-[4px] border-y-transparent ml-0.5" />
          </div>
        </div>
      )}
    </div>
  );

  return (
    <>
      <div
        className={cn(
          'relative overflow-hidden rounded-lg bg-gradient-to-br from-slate-700 to-slate-800 shadow-lg flex-shrink-0',
          className,
          isPlaying && !isMinimized && 'ring-2 ring-purple-500/50 shadow-purple-500/25',
          'transition-all duration-300 ease-in-out',
          // Hover effects if expandable
          ((showExpandModal && artworkUrl) || onExpandPlayer) &&
            'hover:ring-2 hover:ring-purple-400/60 cursor-pointer hover:shadow-xl hover:scale-105'
        )}
        onClick={handleClick}
        data-tooltip-id={
          !isMinimized && ((showExpandModal && artworkUrl) || onExpandPlayer)
            ? 'album-artwork-tooltip'
            : undefined
        }
        data-tooltip-content={
          showExpandModal && artworkUrl ? 'Click to view full size' : 'Click to expand player'
        }
      >
        {artworkElement}
      </div>

      {/* Modal for expanded artwork - render at document root using portal */}
      {showModal &&
        createPortal(
          <ImageModal
            isOpen={showModal}
            onClose={() => setShowModal(false)}
            imageUrl={artworkUrl || ''}
            altText={altText}
            title={
              artistName && albumName && trackTitle
                ? `${artistName} - ${albumName} (${trackTitle})`
                : artistName && albumName
                  ? `${artistName} - ${albumName}`
                  : trackTitle
                    ? trackTitle
                    : altText
            }
          />,
          document.body
        )}

      {/* Tooltip */}
      {!isMinimized && ((showExpandModal && artworkUrl) || onExpandPlayer) && (
        <Tooltip id="album-artwork-tooltip" />
      )}
    </>
  );
};

export default AlbumArtwork;
