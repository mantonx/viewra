import React from 'react';
import { cn } from '../../lib/utils';

interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  icon: React.ReactNode;
  variant?: 'primary' | 'ghost' | 'control';
}

const IconButton: React.FC<IconButtonProps> = ({
  icon,
  variant = 'ghost',
  className,
  ...props
}) => {
  const base = 'flex items-center justify-center rounded-full p-2 transition-colors';

  const variants: Record<typeof variant, string> = {
    primary: 'bg-purple-600 hover:bg-purple-700 text-white',
    ghost: 'bg-transparent hover:bg-slate-700 text-slate-300',
    control: 'bg-slate-800 hover:bg-slate-700 text-white border border-slate-600',
  };

  return (
    <button {...props} className={cn(base, variants[variant], className)}>
      {icon}
    </button>
  );
};

export default IconButton;
