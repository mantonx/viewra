import { describe, it, expect } from 'vitest';
import {
  formatTime,
  formatProgress,
  parseTimeString,
  clampTime,
  getRemainingTime,
  formatRemainingTime,
  isValidTime,
  roundTime,
  formatTooltipTime,
} from './time';

describe('Time Utilities', () => {
  describe('formatTime', () => {
    it('formats seconds to MM:SS format', () => {
      expect(formatTime(0)).toBe('0:00');
      expect(formatTime(59)).toBe('0:59');
      expect(formatTime(60)).toBe('1:00');
      expect(formatTime(125)).toBe('2:05');
    });

    it('formats to HH:MM:SS when hours are present', () => {
      expect(formatTime(3600)).toBe('1:00:00');
      expect(formatTime(3661)).toBe('1:01:01');
      expect(formatTime(7325)).toBe('2:02:05');
    });

    it('handles invalid inputs', () => {
      expect(formatTime(NaN)).toBe('0:00');
      expect(formatTime(Infinity)).toBe('0:00');
      expect(formatTime(-10)).toBe('0:00');
    });
  });

  describe('formatProgress', () => {
    it('calculates correct progress percentage', () => {
      expect(formatProgress(0, 100)).toBe(0);
      expect(formatProgress(50, 100)).toBe(50);
      expect(formatProgress(100, 100)).toBe(100);
      expect(formatProgress(25, 50)).toBe(50);
    });

    it('clamps values between 0 and 100', () => {
      expect(formatProgress(150, 100)).toBe(100);
      expect(formatProgress(-10, 100)).toBe(0);
    });

    it('handles edge cases', () => {
      expect(formatProgress(50, 0)).toBe(0);
      expect(formatProgress(NaN, 100)).toBe(0);
      expect(formatProgress(50, NaN)).toBe(0);
      expect(formatProgress(Infinity, 100)).toBe(0);
    });
  });

  describe('parseTimeString', () => {
    it('parses MM:SS format', () => {
      expect(parseTimeString('0:00')).toBe(0);
      expect(parseTimeString('1:30')).toBe(90);
      expect(parseTimeString('10:45')).toBe(645);
    });

    it('parses HH:MM:SS format', () => {
      expect(parseTimeString('1:00:00')).toBe(3600);
      expect(parseTimeString('2:30:45')).toBe(9045);
      expect(parseTimeString('0:05:30')).toBe(330);
    });

    it('handles invalid inputs', () => {
      expect(parseTimeString('')).toBe(0);
      expect(parseTimeString('invalid')).toBe(0);
      expect(parseTimeString('1:2:3:4')).toBe(0);
    });
  });

  describe('clampTime', () => {
    it('clamps time within valid range', () => {
      expect(clampTime(50, 100)).toBe(50);
      expect(clampTime(0, 100)).toBe(0);
      expect(clampTime(100, 100)).toBe(100);
    });

    it('clamps values outside range', () => {
      expect(clampTime(-10, 100)).toBe(0);
      expect(clampTime(150, 100)).toBe(100);
    });

    it('handles invalid inputs', () => {
      expect(clampTime(NaN, 100)).toBe(0);
      expect(clampTime(50, NaN)).toBe(0);
      expect(clampTime(Infinity, 100)).toBe(0);
    });
  });

  describe('getRemainingTime', () => {
    it('calculates remaining time correctly', () => {
      expect(getRemainingTime(0, 100)).toBe(100);
      expect(getRemainingTime(30, 100)).toBe(70);
      expect(getRemainingTime(100, 100)).toBe(0);
    });

    it('handles edge cases', () => {
      expect(getRemainingTime(150, 100)).toBe(0);
      expect(getRemainingTime(50, 0)).toBe(0);
      expect(getRemainingTime(NaN, 100)).toBe(0);
    });
  });

  describe('formatRemainingTime', () => {
    it('formats remaining time with minus sign', () => {
      expect(formatRemainingTime(0, 100)).toBe('-1:40');
      expect(formatRemainingTime(30, 100)).toBe('-1:10');
      expect(formatRemainingTime(100, 100)).toBe('-0:00');
    });
  });

  describe('isValidTime', () => {
    it('validates correct time values', () => {
      expect(isValidTime(0)).toBe(true);
      expect(isValidTime(100)).toBe(true);
      expect(isValidTime(0.5)).toBe(true);
    });

    it('rejects invalid time values', () => {
      expect(isValidTime(NaN)).toBe(false);
      expect(isValidTime(Infinity)).toBe(false);
      expect(isValidTime(-Infinity)).toBe(false);
      expect(isValidTime(-1)).toBe(false);
    });
  });

  describe('roundTime', () => {
    it('rounds time to nearest second', () => {
      expect(roundTime(10.2)).toBe(10);
      expect(roundTime(10.5)).toBe(11);
      expect(roundTime(10.8)).toBe(11);
      expect(roundTime(0)).toBe(0);
    });
  });

  describe('formatTooltipTime', () => {
    it('formats tooltip time without hours by default', () => {
      expect(formatTooltipTime(125)).toBe('2:05');
      expect(formatTooltipTime(3600)).toBe('1:00:00');
    });

    it('shows hours when requested', () => {
      expect(formatTooltipTime(125, true)).toBe('0:02:05');
      expect(formatTooltipTime(0, true)).toBe('0:00:00');
    });

    it('handles invalid inputs', () => {
      expect(formatTooltipTime(NaN)).toBe('0:00');
      expect(formatTooltipTime(-10)).toBe('0:00');
    });
  });
});