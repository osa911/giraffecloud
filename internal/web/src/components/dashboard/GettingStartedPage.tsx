"use client";

import {
  Box,
  Card,
  CardContent,
  CardHeader,
  Stack,
  Divider,
  Paper,
  IconButton,
  Tooltip,
  Button,
  Typography,
} from "@mui/material";
import {
  Download as DownloadIcon,
  Key as KeyIcon,
  VpnLock as TunnelIcon,
  PlayArrow as PlayArrowIcon,
  ContentCopy as ContentCopyIcon,
  Check as CheckIcon,
} from "@mui/icons-material";
import { useState } from "react";
import toast from "react-hot-toast";
import TunnelDialog from "./tunnels/TunnelDialog";
import TokenDialog from "./settings/TokenDialog";
import { useTunnels } from "@/hooks/useTunnels";

export default function GettingStartedPage() {
  const { mutate, tunnels } = useTunnels();
  const [copiedStep, setCopiedStep] = useState<number | null>(null);

  // Token creation dialog state
  const [tokenDialogOpen, setTokenDialogOpen] = useState(false);
  const [newTokenValue, setNewTokenValue] = useState<string | null>(null);

  // Tunnel creation dialog state
  const [tunnelDialogOpen, setTunnelDialogOpen] = useState(false);

  const handleCopy = (text: string, stepNumber: number) => {
    navigator.clipboard.writeText(text);
    setCopiedStep(stepNumber);
    toast.success("Copied to clipboard!");
    setTimeout(() => setCopiedStep(null), 2000);
  };

  const handleTokenCreated = (tokenValue: string) => {
    setNewTokenValue(tokenValue);
  };

  const handleCloseTokenDialog = () => {
    setTokenDialogOpen(false);
  };

  const installCommand = `curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash`;
  const windowsInstallCommand = `iwr -useb https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1 | iex`;

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Getting Started
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ mb: 3 }}>
        Follow these steps to start using GiraffeCloud and expose your local applications to the
        internet.
      </Typography>

      <Card>
        <CardHeader title="Quick Start Guide" subheader="Set up your tunnel in 5 easy steps" />
        <CardContent>
          <Stack spacing={3}>
            {/* Step 1: Install CLI */}
            <Box>
              <Stack direction="row" spacing={2} alignItems="flex-start">
                <Box
                  sx={{
                    width: 32,
                    height: 32,
                    borderRadius: "50%",
                    bgcolor: "primary.main",
                    color: "white",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontWeight: "bold",
                    flexShrink: 0,
                  }}
                >
                  1
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
                    <DownloadIcon color="primary" />
                    <Typography variant="h6">Install the CLI</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Run this command in your terminal to install GiraffeCloud CLI:
                  </Typography>
                  <Paper
                    variant="outlined"
                    sx={{ p: 2, bgcolor: "background.default", position: "relative" }}
                  >
                    <Typography
                      component="pre"
                      sx={{
                        fontFamily: "monospace",
                        fontSize: "0.875rem",
                        m: 0,
                        overflow: "auto",
                        pr: 5,
                      }}
                    >
                      {installCommand}
                    </Typography>
                    <Tooltip title={copiedStep === 1 ? "Copied!" : "Copy command"}>
                      <IconButton
                        size="small"
                        onClick={() => handleCopy(installCommand, 1)}
                        sx={{ position: "absolute", top: 8, right: 8 }}
                        color={copiedStep === 1 ? "success" : "default"}
                      >
                        {copiedStep === 1 ? <CheckIcon /> : <ContentCopyIcon />}
                      </IconButton>
                    </Tooltip>
                  </Paper>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    sx={{ mt: 1, display: "block" }}
                  >
                    Windows users: Use PowerShell
                  </Typography>
                  <Paper
                    variant="outlined"
                    sx={{ p: 2, bgcolor: "background.default", position: "relative", mt: 1 }}
                  >
                    <Typography
                      component="pre"
                      sx={{
                        fontFamily: "monospace",
                        fontSize: "0.875rem",
                        m: 0,
                        overflow: "auto",
                        pr: 5,
                      }}
                    >
                      {windowsInstallCommand}
                    </Typography>
                    <Tooltip title={copiedStep === 5 ? "Copied!" : "Copy command"}>
                      <IconButton
                        size="small"
                        onClick={() => handleCopy(windowsInstallCommand, 5)}
                        sx={{ position: "absolute", top: 8, right: 8 }}
                        color={copiedStep === 5 ? "success" : "default"}
                      >
                        {copiedStep === 5 ? <CheckIcon /> : <ContentCopyIcon />}
                      </IconButton>
                    </Tooltip>
                  </Paper>
                </Box>
              </Stack>
            </Box>

            <Divider />

            {/* Step 2: Create API Token */}
            <Box>
              <Stack direction="row" spacing={2} alignItems="flex-start">
                <Box
                  sx={{
                    width: 32,
                    height: 32,
                    borderRadius: "50%",
                    bgcolor: "primary.main",
                    color: "white",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontWeight: "bold",
                    flexShrink: 0,
                  }}
                >
                  2
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
                    <KeyIcon color="primary" />
                    <Typography variant="h6">Create an API Token</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Generate a token to authenticate your CLI with GiraffeCloud via secure mutual
                    TLS (mTLS) authentication.
                  </Typography>
                  <Button
                    variant="contained"
                    onClick={() => setTokenDialogOpen(true)}
                    startIcon={<KeyIcon />}
                  >
                    Create Token
                  </Button>
                </Box>
              </Stack>
            </Box>

            <Divider />

            {/* Step 3: Login with API Token */}
            <Box>
              <Stack direction="row" spacing={2} alignItems="flex-start">
                <Box
                  sx={{
                    width: 32,
                    height: 32,
                    borderRadius: "50%",
                    bgcolor: "primary.main",
                    color: "white",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontWeight: "bold",
                    flexShrink: 0,
                  }}
                >
                  3
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
                    <DownloadIcon color="primary" />
                    <Typography variant="h6">Login and Download Certificates</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Use your API token to login and download mTLS certificates:
                  </Typography>
                  <Paper
                    variant="outlined"
                    sx={{ p: 2, bgcolor: "background.default", position: "relative" }}
                  >
                    <Typography
                      component="pre"
                      sx={{
                        fontFamily: "monospace",
                        fontSize: "0.875rem",
                        m: 0,
                        overflow: "auto",
                        pr: 5,
                      }}
                    >
                      {newTokenValue
                        ? `giraffecloud login --token ${"*".repeat(40)}`
                        : `giraffecloud login --token YOUR_API_TOKEN`}
                    </Typography>
                    <Tooltip title={copiedStep === 3 ? "Copied!" : "Copy command"}>
                      <IconButton
                        size="small"
                        onClick={() =>
                          handleCopy(
                            newTokenValue
                              ? `giraffecloud login --token ${newTokenValue}`
                              : "giraffecloud login --token YOUR_API_TOKEN",
                            3,
                          )
                        }
                        sx={{ position: "absolute", top: 8, right: 8 }}
                        color={copiedStep === 3 ? "success" : "default"}
                      >
                        {copiedStep === 3 ? <CheckIcon /> : <ContentCopyIcon />}
                      </IconButton>
                    </Tooltip>
                  </Paper>
                </Box>
              </Stack>
            </Box>

            <Divider />

            {/* Step 4: Create a Tunnel */}
            <Box>
              <Stack direction="row" spacing={2} alignItems="flex-start">
                <Box
                  sx={{
                    width: 32,
                    height: 32,
                    borderRadius: "50%",
                    bgcolor: "primary.main",
                    color: "white",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontWeight: "bold",
                    flexShrink: 0,
                  }}
                >
                  4
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
                    <TunnelIcon color="primary" />
                    <Typography variant="h6">Create a Tunnel</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Create a tunnel with your domain or use our free subdomain. Save the tunnel
                    token that will be displayed.
                  </Typography>
                  <Button
                    variant="contained"
                    onClick={() => setTunnelDialogOpen(true)}
                    startIcon={<TunnelIcon />}
                  >
                    Create Tunnel
                  </Button>
                </Box>
              </Stack>
            </Box>

            <Divider />

            {/* Step 5: Connect - Two Options */}
            <Box>
              <Stack direction="row" spacing={2} alignItems="flex-start">
                <Box
                  sx={{
                    width: 32,
                    height: 32,
                    borderRadius: "50%",
                    bgcolor: "primary.main",
                    color: "white",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontWeight: "bold",
                    flexShrink: 0,
                  }}
                >
                  5
                </Box>
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
                    <PlayArrowIcon color="primary" />
                    <Typography variant="h6">Connect Your Tunnel</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Choose how to run your tunnel:
                  </Typography>

                  {/* Option 1: Direct Connection */}
                  <Box sx={{ mb: 3 }}>
                    <Typography variant="subtitle2" sx={{ mb: 1 }}>
                      Option 1: Direct Connection (Foreground)
                    </Typography>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      display="block"
                      sx={{ mb: 1 }}
                    >
                      Run the tunnel directly in your terminal:
                    </Typography>
                    <Paper
                      variant="outlined"
                      sx={{ p: 2, bgcolor: "background.default", position: "relative" }}
                    >
                      <Typography
                        component="pre"
                        sx={{
                          fontFamily: "monospace",
                          fontSize: "0.875rem",
                          m: 0,
                          overflow: "auto",
                          pr: 5,
                        }}
                      >
                        giraffecloud connect
                      </Typography>
                      <Tooltip title={copiedStep === 5 ? "Copied!" : "Copy command"}>
                        <IconButton
                          size="small"
                          onClick={() => handleCopy("giraffecloud connect", 5)}
                          sx={{ position: "absolute", top: 8, right: 8 }}
                          color={copiedStep === 5 ? "success" : "default"}
                        >
                          {copiedStep === 5 ? <CheckIcon /> : <ContentCopyIcon />}
                        </IconButton>
                      </Tooltip>
                    </Paper>
                  </Box>

                  {/* Option 2: Install as Service */}
                  <Box>
                    <Typography variant="subtitle2" sx={{ mb: 1 }}>
                      Option 2: Install as System Service (Recommended for servers)
                    </Typography>
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      display="block"
                      sx={{ mb: 1 }}
                    >
                      Install and run as a background service (auto-start on boot):
                    </Typography>
                    <Paper
                      variant="outlined"
                      sx={{ p: 2, bgcolor: "background.default", position: "relative", mb: 1 }}
                    >
                      <Typography
                        component="pre"
                        sx={{
                          fontFamily: "monospace",
                          fontSize: "0.875rem",
                          m: 0,
                          overflow: "auto",
                          pr: 5,
                        }}
                      >
                        giraffecloud service install
                      </Typography>
                      <Tooltip title={copiedStep === 6 ? "Copied!" : "Copy command"}>
                        <IconButton
                          size="small"
                          onClick={() => handleCopy("giraffecloud service install", 6)}
                          sx={{ position: "absolute", top: 8, right: 8 }}
                          color={copiedStep === 6 ? "success" : "default"}
                        >
                          {copiedStep === 6 ? <CheckIcon /> : <ContentCopyIcon />}
                        </IconButton>
                      </Tooltip>
                    </Paper>
                    <Paper
                      variant="outlined"
                      sx={{ p: 2, bgcolor: "background.default", position: "relative" }}
                    >
                      <Typography
                        component="pre"
                        sx={{
                          fontFamily: "monospace",
                          fontSize: "0.875rem",
                          m: 0,
                          overflow: "auto",
                          pr: 5,
                        }}
                      >
                        giraffecloud service start
                      </Typography>
                      <Tooltip title={copiedStep === 7 ? "Copied!" : "Copy command"}>
                        <IconButton
                          size="small"
                          onClick={() => handleCopy("giraffecloud service start", 7)}
                          sx={{ position: "absolute", top: 8, right: 8 }}
                          color={copiedStep === 7 ? "success" : "default"}
                        >
                          {copiedStep === 7 ? <CheckIcon /> : <ContentCopyIcon />}
                        </IconButton>
                      </Tooltip>
                    </Paper>
                  </Box>
                </Box>
              </Stack>
            </Box>
          </Stack>
        </CardContent>
      </Card>

      {/* CLI Commands Reference */}
      <Card sx={{ mt: 3 }}>
        <CardHeader
          title="CLI Commands Reference"
          subheader="Complete list of available commands"
        />
        <CardContent>
          <Stack spacing={2}>
            {/* Core Commands */}
            <Box>
              <Typography variant="h6" gutterBottom>
                Core Commands
              </Typography>
              <Paper variant="outlined" sx={{ p: 2 }}>
                <Stack spacing={1.5}>
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud login --token &lt;YOUR_TOKEN&gt;
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Login and download client certificates for secure mutual TLS authentication
                    </Typography>
                    <Typography
                      variant="caption"
                      display="block"
                      sx={{ fontFamily: "monospace", mt: 0.5 }}
                    >
                      Options: --api-host &lt;host&gt;, --api-port &lt;port&gt;
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud connect
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Connect to GiraffeCloud and establish a tunnel to expose your local service
                    </Typography>
                    <Typography
                      variant="caption"
                      display="block"
                      sx={{ fontFamily: "monospace", mt: 0.5 }}
                    >
                      Options: --local-port &lt;port&gt;, --tunnel-host &lt;host&gt;, --tunnel-port
                      &lt;port&gt;
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud status
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Check tunnel connection status and configuration
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud version
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Display version information
                    </Typography>
                  </Box>
                </Stack>
              </Paper>
            </Box>

            {/* Service Management */}
            <Box>
              <Typography variant="h6" gutterBottom>
                Service Management
              </Typography>
              <Paper variant="outlined" sx={{ p: 2 }}>
                <Stack spacing={1.5}>
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud service install
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Install GiraffeCloud as a system service (auto-start on boot)
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud service start
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Start the GiraffeCloud service
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud service stop
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Stop the GiraffeCloud service
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud service restart
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Restart the GiraffeCloud service
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud service uninstall
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Uninstall the GiraffeCloud system service
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud service health-check
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Run comprehensive health check (certificates, connectivity, configuration)
                    </Typography>
                  </Box>
                </Stack>
              </Paper>
            </Box>

            {/* Update Commands */}
            <Box>
              <Typography variant="h6" gutterBottom>
                Update Commands
              </Typography>
              <Paper variant="outlined" sx={{ p: 2 }}>
                <Stack spacing={1.5}>
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud update
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Check for and install updates to the CLI client
                    </Typography>
                    <Typography
                      variant="caption"
                      display="block"
                      sx={{ fontFamily: "monospace", mt: 0.5 }}
                    >
                      Options: --check-only, --force
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud auto-update status
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Show auto-update configuration and status
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud auto-update enable
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Enable automatic updates
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud auto-update disable
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Disable automatic updates
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud auto-update config
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Configure auto-update settings
                    </Typography>
                    <Typography
                      variant="caption"
                      display="block"
                      sx={{ fontFamily: "monospace", mt: 0.5 }}
                    >
                      Options: --interval &lt;duration&gt;, --required-only, --preserve-connection,
                      --restart-service
                    </Typography>
                  </Box>
                </Stack>
              </Paper>
            </Box>

            {/* Configuration Commands */}
            <Box>
              <Typography variant="h6" gutterBottom>
                Configuration Commands
              </Typography>
              <Paper variant="outlined" sx={{ p: 2 }}>
                <Stack spacing={1.5}>
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud config show
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Display current configuration in JSON format
                    </Typography>
                  </Box>
                  <Divider />
                  <Box>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace", fontWeight: "bold" }}
                    >
                      giraffecloud config path
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Show paths to configuration file, certificates, and logs
                    </Typography>
                  </Box>
                </Stack>
              </Paper>
            </Box>
          </Stack>
        </CardContent>
      </Card>

      {/* Token Creation Dialog */}
      <TokenDialog
        open={tokenDialogOpen}
        onClose={handleCloseTokenDialog}
        onSuccess={handleTokenCreated}
      />

      {/* Tunnel Creation Dialog */}
      <TunnelDialog
        open={tunnelDialogOpen}
        onClose={() => setTunnelDialogOpen(false)}
        onSuccess={() => {
          mutate();
          toast.success(
            "Tunnel created! Make sure to copy the tunnel token - you'll need it to connect.",
          );
        }}
        existingTunnels={tunnels}
      />
    </Box>
  );
}
