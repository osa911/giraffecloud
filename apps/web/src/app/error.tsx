"use client";

import { useEffect } from "react";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <div className="flex h-[calc(100vh-4rem)] flex-col items-center justify-center gap-4 text-center">
      <div className="flex h-20 w-20 items-center justify-center rounded-full bg-destructive/10">
        <AlertTriangle className="h-10 w-10 text-destructive" />
      </div>
      <h2 className="text-2xl font-bold tracking-tight">Something went wrong!</h2>
      <p className="text-muted-foreground max-w-[500px]">
        {error.message === "fetch failed" ||
         (error as any).code === "ECONNREFUSED" ||
         JSON.stringify(error).includes("ECONNREFUSED")
          ? "We are unable to connect to the server. The backend service might be down or unreachable."
          : "We apologize for the inconvenience. Please try again."}
      </p>
      <div className="flex gap-4">
        <Button onClick={() => window.location.href = "/"}>Go Home</Button>
        <Button variant="outline" onClick={() => reset()}>
          Try again
        </Button>
      </div>
    </div>
  );
}
