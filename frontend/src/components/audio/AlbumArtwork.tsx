import React, { useState } from 'react';
import { createPortal } from 'react-dom';
import { cn } from '@/lib/utils';
import { Tooltip } from 'react-tooltip';
import ImageModal from '@/components/ui/ImageModal';
import { buildImageUrl } from '@/utils/api';

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
  className,
  showExpandModal = true,
  trackTitle,
  artistName,
  albumName,
}) => {
  const [showModal, setShowModal] = useState(false);

  const handleClick = () => {
    if (showExpandModal && artworkUrl) {
      setShowModal(true);
    } else if (onExpandPlayer) {
      onExpandPlayer();
    }
  };

  const artworkElement = (
    <div className="relative w-full h-full group">
      {/* Main artwork or fallback */}
      {artworkUrl ? (
        <img
          src={buildImageUrl(artworkUrl)}
          alt={altText}
          className="w-full h-full object-cover transition-transform duration-200 group-hover:scale-105"
        />
      ) : (
        <div className="w-full h-full bg-gradient-to-br from-slate-700 to-slate-800 flex items-center justify-center">
          <span className={cn('transition-all', isMinimized ? 'text-xs' : 'text-sm sm:text-lg')}>
            🎵
          </span>
        </div>
      )}

      {/* Subtle reflection/shine overlay */}
      <div className="absolute inset-0 bg-gradient-to-br from-white/10 via-transparent to-black/10 pointer-events-none" />
    </div>
  );

  return (
    <>
      <div
        className={cn(
          'flex-shrink-0 transition-all duration-300 ease-in-out relative overflow-hidden shadow-lg',
          // Base sizing - made smaller and more responsive
          isMinimized ? 'w-8 h-8' : 'w-10 h-10 min-w-10 sm:w-12 sm:h-12 sm:min-w-12',
          // Playing state with soft glow
          isPlaying
            ? 'ring-2 ring-purple-500/70 shadow-purple-500/20 animate-pulse-slow'
            : 'ring-1 ring-white/10',
          // Rounded corners with glassmorphism
          'rounded-lg backdrop-blur-sm',
          // Hover effects if expandable
          ((showExpandModal && artworkUrl) || onExpandPlayer) &&
            'hover:ring-2 hover:ring-purple-400/60 cursor-pointer hover:shadow-xl hover:scale-105',
          className
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
