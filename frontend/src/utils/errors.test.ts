import { describe, it, expect } from 'vitest';
import axios, { AxiosError } from 'axios';
import { getErrorMessage } from './errors';

describe('getErrorMessage', () => {
  it('uses generic status message when response lacks error code', () => {
    const err = new AxiosError('Request failed', '404', undefined, undefined, {
      status: 404,
      statusText: 'Not Found',
      headers: {},
      config: { headers: new axios.AxiosHeaders() },
      data: { error: 'User not found' },
    });
    expect(getErrorMessage(err)).toBe('The requested resource was not found.');
  });

  it('extracts error when response has both code and error fields', () => {
    const err = new AxiosError('Request failed', '400', undefined, undefined, {
      status: 400,
      statusText: 'Bad Request',
      headers: {},
      config: { headers: new axios.AxiosHeaders() },
      data: { code: 'INVALID_EMAIL', error: 'Email format is invalid' },
    });
    expect(getErrorMessage(err)).toBe('Email format is invalid');
  });

  it('uses generic status message for string response data', () => {
    const err = new AxiosError('Request failed', '400', undefined, undefined, {
      status: 400,
      statusText: 'Bad Request',
      headers: {},
      config: { headers: new axios.AxiosHeaders() },
      data: 'Raw error string',
    });
    expect(getErrorMessage(err)).toBe('Invalid request. Please check your input.');
  });

  it('handles 429 rate limit without error field', () => {
    const err = new AxiosError('Too Many Requests', '429', undefined, undefined, {
      status: 429,
      statusText: 'Too Many Requests',
      headers: {},
      config: { headers: new axios.AxiosHeaders() },
      data: {},
    });
    expect(getErrorMessage(err)).toBe('Too many requests. Please try again later.');
  });

  it('returns message from Error instances', () => {
    expect(getErrorMessage(new Error('Something broke'))).toBe('Something broke');
  });

  it('returns fallback for unknown types', () => {
    expect(getErrorMessage(null)).toBe('An unexpected error occurred');
    expect(getErrorMessage(42)).toBe('An unexpected error occurred');
    expect(getErrorMessage(undefined)).toBe('An unexpected error occurred');
  });
});
