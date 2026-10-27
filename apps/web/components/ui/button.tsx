import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex min-h-12 items-center justify-center gap-2 rounded border-2 border-ink px-4 py-2 font-sans text-lg font-bold leading-tight transition-[transform,box-shadow] duration-100 ease-[steps(3)] disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-3 focus-visible:outline-offset-2",
  {
    variants: {
      variant: {
        primary:
          "bg-accent text-ink shadow-hard-sm hover:-translate-y-0.5 hover:shadow-hard active:translate-x-[3px] active:translate-y-[3px] active:shadow-none",
        secondary:
          "bg-surface text-ink shadow-hard-sm hover:-translate-y-0.5 hover:shadow-hard active:translate-x-[3px] active:translate-y-[3px] active:shadow-none",
        destructive:
          "bg-accent-2 text-white shadow-hard-sm hover:-translate-y-0.5 hover:shadow-hard active:translate-x-[3px] active:translate-y-[3px] active:shadow-none",
        ghost: "border-transparent bg-transparent hover:bg-accent-3/40",
        link: "min-h-0 border-transparent bg-transparent px-0 py-0 text-ink underline decoration-2 underline-offset-4 hover:text-accent-2",
      },
      size: {
        default: "",
        sm: "min-h-10 px-3 py-1.5 text-base",
        lg: "min-h-14 px-6 text-xl",
        icon: "h-12 w-12 px-0",
      },
    },
    defaultVariants: { variant: "primary", size: "default" },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
  ),
);
Button.displayName = "Button";

export { buttonVariants };
