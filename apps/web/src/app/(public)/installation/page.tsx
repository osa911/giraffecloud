"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Terminal, Download, ExternalLink } from "lucide-react";

export default function InstallationPage() {
  const CodeBlock = ({ code }: { code: string }) => (
    <div className="bg-muted p-4 rounded-md overflow-x-auto font-mono text-sm my-2">
      {code}
    </div>
  );

  return (
    <div className="container mx-auto px-4 py-12 max-w-4xl">
      <h1 className="text-4xl font-bold mb-4">Install GiraffeCloud</h1>
      <p className="text-lg text-muted-foreground mb-8">
        Choose the one-liner for your platform or view the full installation guide.
      </p>

      <div className="grid gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Terminal className="h-5 w-5" />
              Linux / macOS
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <h3 className="font-medium mb-2">Quick Install</h3>
              <p className="text-sm text-muted-foreground mb-2">
                Installs the CLI in your user profile.
              </p>
              <CodeBlock code="curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash" />
            </div>

            <div>
              <h3 className="font-medium mb-2">Install as System Service</h3>
              <p className="text-sm text-muted-foreground mb-2">
                Installs and starts the Linux system service in one step:
              </p>
              <CodeBlock code="curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash -s -- --service system" />
            </div>

            <div>
              <h3 className="font-medium mb-2">Interactive Mode</h3>
              <p className="text-sm text-muted-foreground mb-2">
                Run with prompts for custom configuration:
              </p>
              <CodeBlock code="bash <(curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh)" />
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Download className="h-5 w-5" />
              Windows (PowerShell)
            </CardTitle>
          </CardHeader>
          <CardContent>
            <CodeBlock code="iwr -useb https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1 | iex" />
          </CardContent>
        </Card>

        <div className="flex flex-wrap gap-4 mt-4">
          <Button variant="outline" asChild>
            <a
              href="https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh"
              target="_blank"
              rel="noopener noreferrer"
            >
              View install.sh <ExternalLink className="ml-2 h-4 w-4" />
            </a>
          </Button>
          <Button variant="outline" asChild>
            <a
              href="https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1"
              target="_blank"
              rel="noopener noreferrer"
            >
              View install.ps1 <ExternalLink className="ml-2 h-4 w-4" />
            </a>
          </Button>
          <Button variant="link" asChild>
            <a
              href="https://github.com/osa911/giraffecloud/blob/main/docs/installation.md"
              target="_blank"
              rel="noopener noreferrer"
            >
              Full installation guide <ExternalLink className="ml-2 h-4 w-4" />
            </a>
          </Button>
        </div>

        <div className="mt-12">
          <h2 className="text-3xl font-bold mb-6">Uninstall GiraffeCloud</h2>
          <p className="text-muted-foreground mb-6">
            Remove GiraffeCloud from your system using our uninstall script.
          </p>

          <div className="grid gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Linux / macOS</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <h3 className="font-medium mb-2">Quick Uninstall</h3>
                  <p className="text-sm text-muted-foreground mb-2">
                    Remove binary and service (keeps configuration):
                  </p>
                  <CodeBlock code="curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/uninstall.sh | bash" />
                </div>
                <div>
                  <h3 className="font-medium mb-2">Full Uninstall</h3>
                  <p className="text-sm text-muted-foreground mb-2">
                    Remove everything including configuration and data:
                  </p>
                  <CodeBlock code="curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/uninstall.sh | bash -s -- --remove-data" />
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Windows (PowerShell)</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <p className="text-sm font-medium text-destructive">Run as Administrator</p>
                <div>
                  <h3 className="font-medium mb-2">Quick Uninstall</h3>
                  <CodeBlock code='Invoke-WebRequest -Uri "https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/uninstall.ps1" -OutFile "$env:TEMP\uninstall.ps1"; & "$env:TEMP\uninstall.ps1"' />
                </div>
                <div>
                  <h3 className="font-medium mb-2">Full Uninstall</h3>
                  <CodeBlock code='Invoke-WebRequest -Uri "https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/uninstall.ps1" -OutFile "$env:TEMP\uninstall.ps1"; & "$env:TEMP\uninstall.ps1" -RemoveData' />
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
