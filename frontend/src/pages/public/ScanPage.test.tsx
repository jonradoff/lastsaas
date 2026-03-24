import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

// Must mock matchMedia BEFORE ScanPage is imported, since it evaluates
// window.matchMedia at module scope (for prefersReducedMotion).
// vi.hoisted runs before any imports.
vi.hoisted(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
});

import ScanPage from './ScanPage';

const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function renderScanPage() {
  return render(
    <MemoryRouter>
      <ScanPage />
    </MemoryRouter>
  );
}

describe('ScanPage', () => {
  beforeEach(() => {
    mockNavigate.mockClear();
  });

  it('renders the heading', () => {
    renderScanPage();
    expect(screen.getByText(/how ai-ready/i)).toBeTruthy();
  });

  it('renders the domain input', () => {
    renderScanPage();
    const input = screen.getByPlaceholderText(/shopify store domain/i);
    expect(input).toBeTruthy();
  });

  it('renders the scan button', () => {
    renderScanPage();
    expect(screen.getByText('Scan')).toBeTruthy();
  });

  it('disables scan button when input is empty', () => {
    renderScanPage();
    const button = screen.getByText('Scan');
    expect(button).toBeDisabled();
  });

  it('enables scan button when domain is entered', () => {
    renderScanPage();
    const input = screen.getByPlaceholderText(/shopify store domain/i);
    fireEvent.change(input, { target: { value: 'allbirds.com' } });
    const button = screen.getByText('Scan');
    expect(button).not.toBeDisabled();
  });

  it('navigates to scan results on submit', () => {
    renderScanPage();
    const input = screen.getByPlaceholderText(/shopify store domain/i);
    fireEvent.change(input, { target: { value: 'allbirds.com' } });

    const form = input.closest('form')!;
    fireEvent.submit(form);

    expect(mockNavigate).toHaveBeenCalledWith('/scan/allbirds.com');
  });

  it('strips protocol from domain before navigating', () => {
    renderScanPage();
    const input = screen.getByPlaceholderText(/shopify store domain/i);
    fireEvent.change(input, { target: { value: 'https://allbirds.com/' } });

    const form = input.closest('form')!;
    fireEvent.submit(form);

    expect(mockNavigate).toHaveBeenCalledWith('/scan/allbirds.com');
  });

  it('does not navigate for empty input', () => {
    renderScanPage();
    const input = screen.getByPlaceholderText(/shopify store domain/i);
    const form = input.closest('form')!;
    fireEvent.submit(form);

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('renders sign in link', () => {
    renderScanPage();
    expect(screen.getByText('Sign in')).toBeTruthy();
  });

  it('renders MCPLens branding', () => {
    renderScanPage();
    expect(screen.getByText('MCPLens')).toBeTruthy();
  });
});
