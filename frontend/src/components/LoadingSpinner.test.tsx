import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import LoadingSpinner from './LoadingSpinner';

describe('LoadingSpinner', () => {
  it('renders with default medium size', () => {
    const { container } = render(<LoadingSpinner />);
    const spinner = container.querySelector('.animate-spin');
    expect(spinner).toBeTruthy();
    expect(spinner?.className).toContain('w-8');
    expect(spinner?.className).toContain('h-8');
  });

  it('renders small size', () => {
    const { container } = render(<LoadingSpinner size="sm" />);
    const spinner = container.querySelector('.animate-spin');
    expect(spinner?.className).toContain('w-4');
    expect(spinner?.className).toContain('h-4');
  });

  it('renders large size', () => {
    const { container } = render(<LoadingSpinner size="lg" />);
    const spinner = container.querySelector('.animate-spin');
    expect(spinner?.className).toContain('w-12');
    expect(spinner?.className).toContain('h-12');
  });

  it('applies custom className', () => {
    const { container } = render(<LoadingSpinner className="my-custom-class" />);
    const wrapper = container.firstElementChild;
    expect(wrapper?.className).toContain('my-custom-class');
  });
});
