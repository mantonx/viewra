import { atom } from 'jotai';

// API status atom - for tracking backend connection
export const apiStatusAtom = atom<'loading' | 'connected' | 'error'>('loading');

// Backend message atom
export const backendMessageAtom = atom<string>('');

// Future atoms for media manager:
// export const mediaLibraryAtom = atom([]);
// export const currentUserAtom = atom(null);
// export const playbackStateAtom = atom(null);
