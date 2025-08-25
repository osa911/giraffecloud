"use client";

import { Box, Container, Typography, Card, CardContent, Stack, Button } from "@mui/material";

export default function InstallationPage() {
  return (
    <Container maxWidth="md" sx={{ py: 6 }}>
      <Typography variant="h3" gutterBottom>
        Install GiraffeCloud
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ mb: 4 }}>
        Choose the one-liner for your platform or view the full installation guide.
      </Typography>

      <Stack spacing={3}>
        <Card variant="outlined">
          <CardContent>
            <Typography variant="h6" gutterBottom>
              Linux/macOS - Quick Install
            </Typography>
            <Box
              component="pre"
              sx={{ p: 2, bgcolor: "background.paper", borderRadius: 1, overflow: "auto" }}
            >
              {`curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash`}
            </Box>
            <Typography variant="body2" color="text.secondary">
              Installs the CLI in your user profile. To install and start the Linux system service
              in one step:
            </Typography>
            <Box
              component="pre"
              sx={{ p: 2, bgcolor: "background.paper", borderRadius: 1, overflow: "auto" }}
            >
              {`curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash -s -- --service system`}
            </Box>
          </CardContent>
        </Card>

        <Card variant="outlined">
          <CardContent>
            <Typography variant="h6" gutterBottom>
              Linux/macOS - Interactive (prompts)
            </Typography>
            <Box
              component="pre"
              sx={{ p: 2, bgcolor: "background.paper", borderRadius: 1, overflow: "auto" }}
            >
              {`bash <(curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh)`}
            </Box>
          </CardContent>
        </Card>

        <Card variant="outlined">
          <CardContent>
            <Typography variant="h6" gutterBottom>
              Windows (PowerShell)
            </Typography>
            <Box
              component="pre"
              sx={{ p: 2, bgcolor: "background.paper", borderRadius: 1, overflow: "auto" }}
            >
              {`iwr -useb https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1 | iex`}
            </Box>
          </CardContent>
        </Card>

        <Stack direction="row" spacing={2}>
          <Button
            variant="contained"
            href="https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh"
            target="_blank"
            rel="noopener noreferrer"
          >
            View install.sh
          </Button>
          <Button
            variant="outlined"
            href="https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.ps1"
            target="_blank"
            rel="noopener noreferrer"
          >
            View install.ps1
          </Button>
          <Button
            variant="text"
            href="https://github.com/osa911/giraffecloud/blob/main/docs/installation.md"
            target="_blank"
            rel="noopener noreferrer"
          >
            Full installation guide
          </Button>
        </Stack>
      </Stack>
    </Container>
  );
}
