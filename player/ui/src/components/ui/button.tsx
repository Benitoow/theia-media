import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import * as React from 'react';

import { cn } from '../../lib/utils';

const buttonVariants = cva(
	'inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-full text-sm font-medium transition-[color,background-color,border-color,box-shadow,transform] duration-150 outline-none disabled:pointer-events-none disabled:opacity-45 focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--color-ink)] active:translate-y-px',
	{
		variants: {
			variant: {
				default:
					'border border-[var(--color-bone)] bg-[var(--color-bone)] text-[var(--color-ink)] shadow-[0_.5rem_1.4rem_rgb(0_0_0/.18)] hover:border-[var(--color-parchment)] hover:bg-[var(--color-parchment)] active:shadow-none',
				outline:
					'border border-[var(--color-line)] bg-transparent text-[var(--color-bone)] hover:border-[var(--color-muted)] hover:bg-[var(--color-raised)] active:bg-[var(--color-surface)]',
				ghost: 'border border-transparent bg-transparent text-[var(--color-muted)] hover:bg-white/6 hover:text-[var(--color-bone)]',
				danger: 'rounded-none bg-transparent text-[var(--color-muted)] hover:bg-[#a9342c] hover:text-[var(--color-bone)]',
			},
			size: {
				default: 'min-h-11 px-5',
				sm: 'min-h-10 px-4 text-xs',
				icon: 'size-11 p-0',
				window: 'h-full min-h-0 w-12 rounded-none p-0',
			},
		},
		defaultVariants: { variant: 'default', size: 'default' },
	}
);

export interface ButtonProps
	extends React.ButtonHTMLAttributes<HTMLButtonElement>,
		VariantProps<typeof buttonVariants> {
	asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
	({ className, variant, size, asChild = false, ...props }, ref) => {
		const Comp = asChild ? Slot : 'button';
		return <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />;
	}
);
Button.displayName = 'Button';

export { Button, buttonVariants };
