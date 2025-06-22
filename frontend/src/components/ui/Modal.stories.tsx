import type { Meta, StoryObj } from '@storybook/react-vite';
import Modal from './Modal';
import { useState } from 'react';

const meta = {
  title: 'UI/Modal',
  component: Modal,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    isOpen: {
      control: 'boolean',
      description: 'Whether modal is open',
    },
    onClose: {
      action: 'closed',
      description: 'Called when modal is closed',
    },
    title: {
      control: 'text',
      description: 'Modal title',
    },
    children: {
      control: false,
      description: 'Modal content',
    },
  },
} satisfies Meta<typeof Modal>;

export default meta;
type Story = StoryObj<typeof meta>;

// Interactive wrapper for modal stories
const ModalDemo = ({ title, children }: { title: string; children: React.ReactNode }) => {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <div className="p-4">
      <button
        onClick={() => setIsOpen(true)}
        className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded"
      >
        Open Modal
      </button>
      <Modal isOpen={isOpen} onClose={() => setIsOpen(false)} title={title}>
        {children}
      </Modal>
    </div>
  );
};

export const Default: Story = {
  render: () => (
    <ModalDemo title="Example Modal">
      <p className="text-slate-300">This is the modal content.</p>
    </ModalDemo>
  ),
};

export const WithForm: Story = {
  render: () => (
    <ModalDemo title="Settings">
      <form className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-slate-300 mb-1">
            Name
          </label>
          <input
            type="text"
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-white"
            placeholder="Enter your name"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-slate-300 mb-1">
            Email
          </label>
          <input
            type="email"
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded text-white"
            placeholder="Enter your email"
          />
        </div>
        <div className="flex gap-2 justify-end pt-4">
          <button
            type="button"
            className="px-4 py-2 bg-slate-600 hover:bg-slate-700 text-white rounded"
          >
            Cancel
          </button>
          <button
            type="submit"
            className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded"
          >
            Save
          </button>
        </div>
      </form>
    </ModalDemo>
  ),
};

export const LongContent: Story = {
  render: () => (
    <ModalDemo title="Terms of Service">
      <div className="space-y-4 max-h-96 overflow-y-auto">
        <p className="text-slate-300">
          Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor 
          incididunt ut labore et dolore magna aliqua.
        </p>
        <p className="text-slate-300">
          Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut 
          aliquip ex ea commodo consequat.
        </p>
        <p className="text-slate-300">
          Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore 
          eu fugiat nulla pariatur.
        </p>
        <p className="text-slate-300">
          Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia 
          deserunt mollit anim id est laborum.
        </p>
        <p className="text-slate-300">
          Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor 
          incididunt ut labore et dolore magna aliqua.
        </p>
      </div>
    </ModalDemo>
  ),
};

export const ConfirmDialog: Story = {
  render: () => (
    <ModalDemo title="Confirm Delete">
      <div className="space-y-4">
        <p className="text-slate-300">
          Are you sure you want to delete this item? This action cannot be undone.
        </p>
        <div className="flex gap-2 justify-end">
          <button className="px-4 py-2 bg-slate-600 hover:bg-slate-700 text-white rounded">
            Cancel
          </button>
          <button className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded">
            Delete
          </button>
        </div>
      </div>
    </ModalDemo>
  ),
};

export const NoTitle: Story = {
  render: () => {
    const [isOpen, setIsOpen] = useState(false);

    return (
      <div className="p-4">
        <button
          onClick={() => setIsOpen(true)}
          className="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded"
        >
          Open Modal
        </button>
        <Modal isOpen={isOpen} onClose={() => setIsOpen(false)}>
          <div className="text-center py-8">
            <div className="text-6xl mb-4">✅</div>
            <h3 className="text-xl font-bold text-white mb-2">Success!</h3>
            <p className="text-slate-300">Your changes have been saved.</p>
          </div>
        </Modal>
      </div>
    );
  },
};