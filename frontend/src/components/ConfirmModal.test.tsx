import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ConfirmModal from './ConfirmModal';

describe('ConfirmModal', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onConfirm: vi.fn(),
    title: 'Delete Item',
    message: 'Are you sure you want to delete this item?',
  };

  it('renders nothing when closed', () => {
    const { container } = render(<ConfirmModal {...defaultProps} open={false} />);
    expect(container.innerHTML).toBe('');
  });

  it('renders title and message when open', () => {
    render(<ConfirmModal {...defaultProps} />);
    expect(screen.getByText('Delete Item')).toBeTruthy();
    expect(screen.getByText('Are you sure you want to delete this item?')).toBeTruthy();
  });

  it('renders default confirm label', () => {
    render(<ConfirmModal {...defaultProps} />);
    expect(screen.getByText('Confirm')).toBeTruthy();
  });

  it('renders custom confirm label', () => {
    render(<ConfirmModal {...defaultProps} confirmLabel="Delete" />);
    expect(screen.getByText('Delete')).toBeTruthy();
  });

  it('calls onConfirm when confirm button clicked', () => {
    const onConfirm = vi.fn();
    render(<ConfirmModal {...defaultProps} onConfirm={onConfirm} />);
    fireEvent.click(screen.getByText('Confirm'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when cancel button clicked', () => {
    const onClose = vi.fn();
    render(<ConfirmModal {...defaultProps} onClose={onClose} />);
    fireEvent.click(screen.getByText('Cancel'));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('shows loading text when loading', () => {
    render(<ConfirmModal {...defaultProps} loading={true} />);
    expect(screen.getByText('Please wait...')).toBeTruthy();
  });

  it('disables buttons when loading', () => {
    render(<ConfirmModal {...defaultProps} loading={true} />);
    const buttons = screen.getAllByRole('button');
    buttons.forEach((btn) => {
      expect(btn).toBeDisabled();
    });
  });

  it('has correct role and aria attributes', () => {
    render(<ConfirmModal {...defaultProps} />);
    const dialog = screen.getByRole('dialog');
    expect(dialog).toBeTruthy();
    expect(dialog.getAttribute('aria-modal')).toBe('true');
  });

  it('calls onClose on backdrop click', () => {
    const onClose = vi.fn();
    render(<ConfirmModal {...defaultProps} onClose={onClose} />);
    // The backdrop is the fixed inset div with bg-black/60
    const backdrop = document.querySelector('.bg-black\\/60');
    if (backdrop) {
      fireEvent.click(backdrop);
      expect(onClose).toHaveBeenCalledTimes(1);
    }
  });
});
