import { describe, it, expect, beforeEach } from 'vitest';
import { setAuthToken, setTenantHeader } from './client';
import api from './client';

describe('API client', () => {
  beforeEach(() => {
    // Reset headers
    delete api.defaults.headers.common['Authorization'];
    delete api.defaults.headers.common['X-Tenant-ID'];
  });

  describe('setAuthToken', () => {
    it('sets Bearer token when given a string', () => {
      setAuthToken('test-token');
      expect(api.defaults.headers.common['Authorization']).toBe('Bearer test-token');
    });

    it('removes Authorization header when given null', () => {
      setAuthToken('test-token');
      setAuthToken(null);
      expect(api.defaults.headers.common['Authorization']).toBeUndefined();
    });
  });

  describe('setTenantHeader', () => {
    it('sets X-Tenant-ID when given a string', () => {
      setTenantHeader('tenant-123');
      expect(api.defaults.headers.common['X-Tenant-ID']).toBe('tenant-123');
    });

    it('removes X-Tenant-ID when given null', () => {
      setTenantHeader('tenant-123');
      setTenantHeader(null);
      expect(api.defaults.headers.common['X-Tenant-ID']).toBeUndefined();
    });
  });

  describe('base configuration', () => {
    it('has correct baseURL', () => {
      expect(api.defaults.baseURL).toBe('/api');
    });

    it('has JSON content type', () => {
      expect(api.defaults.headers['Content-Type']).toBe('application/json');
    });
  });
});
