"use client";

import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Download,
  Key,
  Network,
  Play,
  Copy,
  Check,
  Terminal,
  Server,
  RefreshCw,
  Settings,
  Trash,
  Activity,
} from "lucide-react";
import { toast } from "@/lib/toast";
import TunnelDialog from "./tunnels/TunnelDialog";
import TokenDialog from "./settings/TokenDialog";
import { useTunnels } from "@/hooks/useTunnels";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

export default function GettingStartedPage() {
  const { mutate } = useTunnels();
  const [copiedStep, setCopiedStep] = useState<number | string | null>(null);

  // Token creation dialog state
  const [tokenDialogOpen, setTokenDialogOpen] = useState(false);
  const [newTokenValue, setNewTokenValue] = useState<string | null>(null);

  // Tunnel creation dialog state
  const [tunnelDialogOpen, setTunnelDialogOpen] = useState(false);

  const handleCopy = (text: string, stepNumber: number | string) => {
    navigator.clipboard.writeText(text);
    setCopiedStep(stepNumber);
    toast.success("Copied to clipboard!");
    setTimeout(() => setCopiedStep(null), 2000);
  };

  const handleTokenCreated = (tokenValue: string) => {
    setNewTokenValue(tokenValue);
  };

  const installCommand = `curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash`;
  const windowsInstallCommand = `iwr -useb https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1 | iex`;

  const StepNumber = ({ number }: { number: number }) => (
    <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground font-bold">
      {number}
    </div>
  );

  const CodeBlock = ({
    code,
    step,
    label,
  }: {
    code: string;
    step: number | string;
    label?: string;
  }) => (
    <div className="relative mt-2 rounded-md bg-muted p-4">
      {label && (
        <div className="mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wider">
          {label}
        </div>
      )}
      <pre className="overflow-x-auto font-mono text-sm pr-10 whitespace-pre">
        <code>{code}</code>
      </pre>
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className={cn(
                "absolute right-2 top-2 h-8 w-8",
                copiedStep === step && "text-green-500 hover:text-green-600"
              )}
              onClick={() => handleCopy(code, step)}
            >
              {copiedStep === step ? (
                <Check className="h-4 w-4" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            <p>{copiedStep === step ? "Copied!" : "Copy command"}</p>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </div>
  );

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-lg font-medium">Getting Started</h3>
        <p className="text-sm text-muted-foreground">
          Follow these steps to start using GiraffeCloud and expose your local applications to the internet.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Quick Start Guide</CardTitle>
          <CardDescription>Set up your tunnel in 5 easy steps</CardDescription>
        </CardHeader>
        <CardContent className="space-y-8">
          {/* Step 1: Install CLI */}
          <div className="flex gap-4">
            <StepNumber number={1} />
            <div className="flex-1 space-y-2 min-w-0">
              <div className="flex items-center gap-2">
                <Download className="h-5 w-5 text-primary" />
                <h4 className="font-medium">Install the CLI</h4>
              </div>
              <p className="text-sm text-muted-foreground">
                Run this command in your terminal to install GiraffeCloud CLI:
              </p>
              <CodeBlock code={installCommand} step={1} label="Linux / macOS" />
              <CodeBlock code={windowsInstallCommand} step={5} label="Windows (PowerShell)" />
            </div>
          </div>

          <Separator />

          {/* Step 2: Create API Token */}
          <div className="flex gap-4">
            <StepNumber number={2} />
            <div className="flex-1 space-y-2 min-w-0">
              <div className="flex items-center gap-2">
                <Key className="h-5 w-5 text-primary" />
                <h4 className="font-medium">Create an API Token</h4>
              </div>
              <p className="text-sm text-muted-foreground">
                Generate a token to authenticate your CLI with GiraffeCloud via secure mutual TLS (mTLS) authentication.
              </p>
              <Button onClick={() => setTokenDialogOpen(true)}>
                <Key className="mr-2 h-4 w-4" />
                Create Token
              </Button>
            </div>
          </div>

          <Separator />

          {/* Step 3: Login */}
          <div className="flex gap-4">
            <StepNumber number={3} />
            <div className="flex-1 space-y-2 min-w-0">
              <div className="flex items-center gap-2">
                <Terminal className="h-5 w-5 text-primary" />
                <h4 className="font-medium">Login and Download Certificates</h4>
              </div>
              <p className="text-sm text-muted-foreground">
                Use your API token to login and download mTLS certificates:
              </p>
              <CodeBlock
                code={
                  newTokenValue
                    ? `giraffecloud login --token ${newTokenValue}`
                    : `giraffecloud login --token YOUR_API_TOKEN`
                }
                step={3}
              />
            </div>
          </div>

          <Separator />

          {/* Step 4: Create Tunnel */}
          <div className="flex gap-4">
            <StepNumber number={4} />
            <div className="flex-1 space-y-2 min-w-0">
              <div className="flex items-center gap-2">
                <Network className="h-5 w-5 text-primary" />
                <h4 className="font-medium">Create a Tunnel</h4>
              </div>
              <p className="text-sm text-muted-foreground">
                Create a tunnel with your domain or use our free subdomain.
              </p>
              <Button onClick={() => setTunnelDialogOpen(true)}>
                <Network className="mr-2 h-4 w-4" />
                Create Tunnel
              </Button>
            </div>
          </div>

          <Separator />

          {/* Step 5: Connect */}
          <div className="flex gap-4">
            <StepNumber number={5} />
            <div className="flex-1 space-y-4 min-w-0">
              <div className="flex items-center gap-2">
                <Play className="h-5 w-5 text-primary" />
                <h4 className="font-medium">Connect Your Tunnel</h4>
              </div>
              <p className="text-sm text-muted-foreground">
                Choose how to run your tunnel:
              </p>

              <div className="space-y-2">
                <h5 className="text-sm font-medium">Option 1: Direct Connection (Foreground)</h5>
                <p className="text-xs text-muted-foreground">Run the tunnel directly in your terminal:</p>
                <CodeBlock code="giraffecloud connect" step={5} />
                <p className="text-xs text-muted-foreground mt-2">
                  💡 <strong>Tip:</strong> If you have multiple tunnels, specify which one with{" "}
                  <code className="bg-muted px-1 py-0.5 rounded">giraffecloud connect --domain your-domain.com</code>
                </p>
              </div>

              <div className="space-y-2">
                <h5 className="text-sm font-medium">Option 2: Install as System Service (Recommended)</h5>
                <p className="text-xs text-muted-foreground">Install and run as a background service (auto-start on boot):</p>
                <CodeBlock code="giraffecloud service install" step={6} />
                <CodeBlock code="giraffecloud service start" step={7} />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>CLI Commands Reference</CardTitle>
          <CardDescription>Complete list of available commands</CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-4">
            <h4 className="font-medium flex items-center gap-2">
              <Terminal className="h-4 w-4" /> Core Commands
            </h4>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud login --token &lt;TOKEN&gt;</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud login --token <TOKEN>", "cmd-login")}
                  >
                    {copiedStep === "cmd-login" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Login and download client certificates.</p>
              </div>
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud connect</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud connect", "cmd-connect")}
                  >
                    {copiedStep === "cmd-connect" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Connect to GiraffeCloud and establish a tunnel.</p>
              </div>
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud status</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud status", "cmd-status")}
                  >
                    {copiedStep === "cmd-status" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Check tunnel connection status.</p>
              </div>
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud version</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud version", "cmd-version")}
                  >
                    {copiedStep === "cmd-version" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Display version information.</p>
              </div>
            </div>
          </div>

          <Separator />

          <div className="space-y-4">
            <h4 className="font-medium flex items-center gap-2">
              <Server className="h-4 w-4" /> Service Management
            </h4>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud service install</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud service install", "cmd-svc-install")}
                  >
                    {copiedStep === "cmd-svc-install" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Install as system service.</p>
              </div>
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud service start</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud service start", "cmd-svc-start")}
                  >
                    {copiedStep === "cmd-svc-start" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Start the service.</p>
              </div>
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud service stop</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud service stop", "cmd-svc-stop")}
                  >
                    {copiedStep === "cmd-svc-stop" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Stop the service.</p>
              </div>
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud service health-check</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud service health-check", "cmd-svc-health")}
                  >
                    {copiedStep === "cmd-svc-health" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Run comprehensive health check.</p>
              </div>
            </div>
          </div>

          <Separator />

          <div className="space-y-4">
            <h4 className="font-medium flex items-center gap-2">
              <RefreshCw className="h-4 w-4" /> Update Commands
            </h4>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud update</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud update", "cmd-update")}
                  >
                    {copiedStep === "cmd-update" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Check for and install updates.</p>
              </div>
              <div className="rounded-lg border p-4 space-y-2 relative group">
                <div className="flex items-center justify-between gap-2">
                  <code className="text-sm font-bold block break-all">giraffecloud auto-update enable</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    onClick={() => handleCopy("giraffecloud auto-update enable", "cmd-autoupdate")}
                  >
                    {copiedStep === "cmd-autoupdate" ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">Enable automatic updates.</p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <TokenDialog
        open={tokenDialogOpen}
        onOpenChange={setTokenDialogOpen}
        onSuccess={handleTokenCreated}
      />

      <TunnelDialog
        open={tunnelDialogOpen}
        onOpenChange={setTunnelDialogOpen}
        onSuccess={() => mutate()}
      />
    </div>
  );
}
