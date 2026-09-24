import {
	Children,
	createContext,
	isValidElement,
	useContext,
	type HTMLAttributes,
	type ReactElement,
	type ReactNode,
} from 'react';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '../../lib/utils';

// One bar split into proportional segments, each with its own weight, colour
// and legend.
//
// The maintainer brought this shape on 24 September 2026 for the viewing
// sheet's films/series ratio, and it is kept as it arrived: the structure, the
// props and the arithmetic are the reference's. What changed is the palette -
// `bg-primary` and `text-slate-500` are another product's theme, and this one
// has tokens (`--color-parchment`, `--color-muted`, `--color-line`) that the
// rest of the interface already speaks. The OSD does have Tailwind, so the
// utilities below are the reference's own; only the colours are ours.

type PartitionBarContextType = {
	total: number;
	size: VariantProps<typeof partitionBarVariants>['size'];
};

const PartitionBarCtxt = createContext<PartitionBarContextType | null>(null);

function usePartitionBarContext(): PartitionBarContextType {
	const context = useContext(PartitionBarCtxt);
	if (!context) {
		throw new Error('usePartitionBarContext must be used within a PartitionBarProvider');
	}
	return context;
}

const partitionBarVariants = cva('flex w-full flex-row', {
	variants: {
		size: {
			sm: 'text-[length:var(--text-micro)]',
			md: 'text-[length:var(--text-small)]',
			lg: 'text-[length:var(--text-body)]',
		},
	},
	defaultVariants: {
		size: 'md',
	},
});

interface PartitionBar
	extends HTMLAttributes<HTMLUListElement>,
		VariantProps<typeof partitionBarVariants> {
	children?: ReactElement<PartitionBarSegment> | ReactElement<PartitionBarSegment>[];
	gap?: number;
}

export default function PartitionBar({ children, className, gap = 1, size, ...props }: PartitionBar) {
	const total = Children.toArray(children).reduce<number>(
		(sum, child) =>
			isValidElement(child) ? sum + ((child.props as PartitionBarSegment).num || 0) : sum,
		0
	);

	return (
		<PartitionBarCtxt.Provider value={{ total, size }}>
			<ul
				className={cn('m-0 list-none p-0', partitionBarVariants({ size }), className)}
				style={{ gap: `${gap * 4}px` }}
				{...props}
			>
				{children}
			</ul>
		</PartitionBarCtxt.Provider>
	);
}

const partitionBarLineVariants = cva('w-full shrink-0 rounded-full', {
	variants: {
		variant: {
			default: 'bg-[var(--color-parchment)]',
			secondary: 'bg-[var(--color-muted)]',
			muted: 'bg-[var(--color-line)]',
		},
	},
	defaultVariants: {
		variant: 'default',
	},
});

const partitionBarTitleVariants = cva('', {
	variants: {
		variant: {
			default: 'text-[var(--color-parchment)]',
			secondary: 'text-[var(--color-muted)]',
			muted: 'text-[var(--color-muted)]',
		},
	},
	defaultVariants: {
		variant: 'default',
	},
});

interface PartitionBarSegment
	extends HTMLAttributes<HTMLLIElement>,
		VariantProps<typeof partitionBarLineVariants> {
	children?: ReactNode;
	num?: number;
	alignment?: 'left' | 'center' | 'right';
}

export function PartitionBarSegment({
	children,
	num = 0,
	variant = 'default',
	alignment = 'center',
	className,
	...props
}: PartitionBarSegment) {
	const { total, size } = usePartitionBarContext();

	const widthPercent = total > 0 ? (num / total) * 100 : 0;

	return (
		<li
			// `partition-bar-segment` is a hook the render check reads the split
			// from: the widths are inline styles, and a check that had to walk
			// every list in the sheet to find them would be asserting the sheet
			// rather than the bar.
			className="partition-bar-segment flex min-w-0 flex-col"
			style={{ flexBasis: `${widthPercent}%`, flexGrow: 0, flexShrink: 0 }}
			{...props}
		>
			<div
				className={cn(
					partitionBarLineVariants({ variant }),
					size === 'sm' ? 'h-1.5' : size === 'md' ? 'h-2' : 'h-3',
					className
				)}
			/>
			<div
				className={cn(
					partitionBarTitleVariants({ variant }),
					'flex w-full flex-col whitespace-normal',
					size === 'sm' ? 'mt-2' : size === 'md' ? 'mt-3' : 'mt-4',
					alignment === 'left' && 'items-start',
					alignment === 'center' && 'items-center',
					alignment === 'right' && 'items-end'
				)}
			>
				{children}
			</div>
		</li>
	);
}

interface PartitionBarSegmentTitle extends HTMLAttributes<HTMLDivElement> {
	children: ReactNode;
}

export function PartitionBarSegmentTitle({ children, className }: PartitionBarSegmentTitle) {
	return <div className={cn('w-fit font-medium', className)}>{children}</div>;
}

interface PartitionBarSegmentValue extends HTMLAttributes<HTMLDivElement> {
	children: ReactNode;
}

export function PartitionBarSegmentValue({ children, className }: PartitionBarSegmentValue) {
	return (
		<div className={cn('w-fit text-[80%] text-[var(--color-muted)]', className)}>{children}</div>
	);
}
