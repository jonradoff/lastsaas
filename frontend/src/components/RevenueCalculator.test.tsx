import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import RevenueCalculator from './RevenueCalculator';

// Wrap in MemoryRouter since it uses Link
function renderWithRouter() {
  return render(
    <MemoryRouter>
      <RevenueCalculator />
    </MemoryRouter>
  );
}

describe('RevenueCalculator', () => {
  it('renders the component', () => {
    renderWithRouter();
    expect(screen.getByText(/opportunity/i) || screen.getByText(/revenue/i) || screen.getByText(/calculator/i)).toBeTruthy();
  });

  it('has interactive sliders', () => {
    renderWithRouter();
    const sliders = document.querySelectorAll('input[type="range"]');
    expect(sliders.length).toBeGreaterThanOrEqual(2); // revenue + score at minimum
  });

  it('displays a dollar amount', () => {
    renderWithRouter();
    // Should show some dollar amount in the output
    const dollarElements = document.querySelectorAll('[class*="text"]');
    const hasDollar = Array.from(dollarElements).some(el =>
      el.textContent?.includes('$')
    );
    expect(hasDollar).toBe(true);
  });

  it('has a CTA link', () => {
    renderWithRouter();
    const links = document.querySelectorAll('a');
    expect(links.length).toBeGreaterThan(0);
  });
});
