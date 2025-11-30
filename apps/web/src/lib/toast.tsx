import { toast as sonnerToast, ExternalToast } from "sonner";
import { Check, AlertTriangle, Info, AlertCircle } from "lucide-react";

type ToastOptions = ExternalToast;

export const toast = {
  success: (message: string, options?: ToastOptions) => {
    sonnerToast.success(message, {
      ...options,
      style: {
        color: "hsl(var(--success))",
        borderColor: "hsl(var(--success))",
        ...options?.style,
      },
      className: "group-[.toaster]:text-green-600 group-[.toaster]:border-green-600 dark:group-[.toaster]:text-green-400 dark:group-[.toaster]:border-green-400",
      icon: <Check className="h-4 w-4 text-green-600 dark:text-green-400" />,
    });
  },
  error: (message: string, options?: ToastOptions) => {
    sonnerToast.error(message, {
      ...options,
      style: {
        color: "hsl(var(--destructive))",
        borderColor: "hsl(var(--destructive))",
        ...options?.style,
      },
      className: "group-[.toaster]:text-red-600 group-[.toaster]:border-red-600 dark:group-[.toaster]:text-red-400 dark:group-[.toaster]:border-red-400",
      icon: <AlertCircle className="h-4 w-4 text-red-600 dark:text-red-400" />,
    });
  },
  info: (message: string, options?: ToastOptions) => {
    sonnerToast.info(message, {
      ...options,
      icon: <Info className="h-4 w-4 text-blue-600 dark:text-blue-400" />,
    });
  },
  warning: (message: string, options?: ToastOptions) => {
    sonnerToast.warning(message, {
      ...options,
      className: "group-[.toaster]:text-yellow-600 group-[.toaster]:border-yellow-600 dark:group-[.toaster]:text-yellow-400 dark:group-[.toaster]:border-yellow-400",
      icon: <AlertTriangle className="h-4 w-4 text-yellow-600 dark:text-yellow-400" />,
    });
  },
  // Expose raw sonner toast for custom needs
  raw: sonnerToast,
};
