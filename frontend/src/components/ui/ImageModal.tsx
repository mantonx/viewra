import React, { useState, useEffect } from 'react';
import Modal from '@/components/ui/Modal';
import { cn } from '@/lib/utils';
import { extractColorsFromImage, type ColorPalette } from '@/utils/colorExtractor';

interface ImageModalProps {
  isOpen: boolean;
  onClose: () => void;
  imageUrl: string;
  altText?: string;
  title?: string;
}

const ImageModal: React.FC<ImageModalProps> = ({
  isOpen,
  onClose,
  imageUrl,
  altText = 'Image',
  title,
}) => {
  const [colorPalette, setColorPalette] = useState<ColorPalette | null>(null);

  // Extract colors from the image when modal opens
  useEffect(() => {
    if (isOpen && imageUrl) {
      extractColorsFromImage(imageUrl).then(setColorPalette);
    }
  }, [isOpen, imageUrl]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      size="full"
      className="bg-transparent shadow-none"
      overlayClassName="bg-black/85"
      contentClassName="p-0 flex items-center justify-center w-full h-full"
      showCloseButton={false}
    >
      <div className="relative w-full h-full flex items-center justify-center">
        {/* Close button positioned over the image */}
        <button
          onClick={onClose}
          className="absolute top-6 right-6 z-50 p-3 bg-black/60 hover:bg-black/80 rounded-full text-white transition-all duration-200 backdrop-blur-sm shadow-lg hover:shadow-xl hover:scale-110 cursor-pointer"
          aria-label="Close image"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>

        {/* Image with dynamic color glow effect */}
        <div className="relative flex items-center justify-center w-full h-full p-8">
          <img
            src={imageUrl}
            alt={altText}
            className={cn(
              'max-w-[75vw] max-h-[75vh] w-auto h-auto object-contain rounded-xl shadow-2xl',
              'animate-zoom-in',
              'transition-all duration-300',
              'cursor-default'
            )}
          />

          {/* Dynamic color-based glow effects */}
          {colorPalette && (
            <>
              <div
                className="absolute inset-0 -z-10 blur-xl scale-110 rounded-xl opacity-40"
                style={{
                  background: `radial-gradient(ellipse at center, ${colorPalette.primary}40, ${colorPalette.secondary}20, transparent 70%)`,
                }}
              />
              <div
                className="absolute inset-0 -z-20 blur-2xl scale-125 rounded-xl opacity-20"
                style={{
                  background: `radial-gradient(ellipse at center, ${colorPalette.accent}30, ${colorPalette.primary}15, transparent 60%)`,
                }}
              />
            </>
          )}

          {/* Fallback glow if color extraction fails */}
          {!colorPalette && (
            <>
              <div className="absolute inset-0 -z-10 bg-gradient-to-br from-purple-500/20 to-blue-500/20 blur-xl scale-110 rounded-xl opacity-40" />
              <div className="absolute inset-0 -z-20 bg-gradient-to-br from-cyan-500/15 to-purple-500/15 blur-2xl scale-125 rounded-xl opacity-20" />
            </>
          )}
        </div>

        {/* Optional title overlay with better styling */}
        {title && (
          <div className="absolute bottom-6 left-1/2 transform -translate-x-1/2 bg-black/80 backdrop-blur-md rounded-lg px-6 py-3 shadow-lg max-w-[90vw]">
            <p className="text-white text-sm font-medium text-center truncate">{title}</p>
          </div>
        )}
      </div>
    </Modal>
  );
};

export default ImageModal;
