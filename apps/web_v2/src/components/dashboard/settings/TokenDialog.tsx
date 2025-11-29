"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Loader2, Copy, Check, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { createToken } from "@/services/api/tokenApi";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface TokenDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: (tokenValue: string) => void;
}

export default function TokenDialog({ open, onOpenChange, onSuccess }: TokenDialogProps) {
  const [newTokenName, setNewTokenName] = useState("");
  const [newTokenValue, setNewTokenValue] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [loading, setLoading] = useState(false);

  const handleCopyToken = () => {
    if (newTokenValue) {
      navigator.clipboard.writeText(newTokenValue);
      setCopied(true);
      toast.success("Token copied to clipboard!");
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleCreateToken = async () => {
    if (!newTokenName.trim()) return;

    setLoading(true);
    try {
      const data = await createToken(newTokenName);
      setNewTokenValue(data.token);
      toast.success("Token created successfully!");
      onSuccess(data.token);
    } catch (error) {
      console.error("Error creating token:", error);
      toast.error("Failed to create token");
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setNewTokenName("");
    setNewTokenValue(null);
    setCopied(false);
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={(val) => !val && handleClose()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{newTokenValue ? "Save Your Token" : "Create New Token"}</DialogTitle>
          <DialogDescription>
            {newTokenValue
              ? "Make sure to copy your token now. You won't be able to see it again!"
              : "Generate a new API token for accessing the GiraffeCloud API."}
          </DialogDescription>
        </DialogHeader>

        {newTokenValue ? (
          <div className="space-y-4">
            <Alert variant="destructive">
              <AlertTriangle className="h-4 w-4" />
              <AlertTitle>Warning</AlertTitle>
              <AlertDescription>
                This token will only be shown once. Copy it now!
              </AlertDescription>
            </Alert>

            <div className="flex items-center space-x-2">
              <Input
                value={newTokenValue}
                readOnly
                className="font-mono text-sm"
              />
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="icon"
                      onClick={handleCopyToken}
                      className={copied ? "text-green-500 border-green-500" : ""}
                    >
                      {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>{copied ? "Copied!" : "Copy to clipboard"}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
          </div>
        ) : (
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="token-name">Token Name</Label>
              <Input
                id="token-name"
                placeholder="e.g., My Laptop, CI Server"
                value={newTokenName}
                onChange={(e) => setNewTokenName(e.target.value)}
                disabled={loading}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && newTokenName.trim()) {
                    handleCreateToken();
                  }
                }}
              />
              <p className="text-xs text-muted-foreground">
                Give your token a descriptive name to identify it later.
              </p>
            </div>
          </div>
        )}

        <DialogFooter>
          {newTokenValue ? (
            <Button onClick={handleClose}>Close</Button>
          ) : (
            <>
              <Button variant="outline" onClick={handleClose} disabled={loading}>
                Cancel
              </Button>
              <Button
                onClick={handleCreateToken}
                disabled={loading || !newTokenName.trim()}
              >
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Create
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
