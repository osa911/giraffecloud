"use client";

import { use, useState } from "react";
import { useRouter } from "next/navigation";
import {
  Box,
  Typography,
  Button,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  IconButton,
  CircularProgress,
} from "@mui/material";
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Refresh as RefreshIcon,
} from "@mui/icons-material";
import { api } from "@/services/api";
import { Tunnel, ApiResponse } from "@/types";

// Function to fetch tunnels
function fetchTunnels() {
  return api
    .get<ApiResponse<Tunnel[]>>("/tunnels")
    .then((res) => res.data.data);
}

export default function TunnelsPage() {
  const router = useRouter();
  const [tunnelPromise, setTunnelPromise] = useState(() => fetchTunnels());
  // Using React 19's 'use' hook for data fetching
  const tunnels = use(tunnelPromise);
  const [isDeleting, setIsDeleting] = useState<string | null>(null);

  // Function to refresh tunnels
  function handleRefresh() {
    setTunnelPromise(fetchTunnels());
  }

  // Function to delete a tunnel
  async function handleDelete(id: string) {
    setIsDeleting(id);
    try {
      await api.delete(`/tunnels/${id}`);
      handleRefresh();
    } catch (error) {
      // Error is handled by API interceptor
    } finally {
      setIsDeleting(null);
    }
  }

  // Function to get status color
  function getStatusColor(status: string): "success" | "error" | "warning" {
    switch (status) {
      case "online":
        return "success";
      case "offline":
        return "error";
      default:
        return "warning";
    }
  }

  return (
    <Box>
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          mb: 3,
        }}
      >
        <Typography variant="h4">Tunnels</Typography>
        <Box sx={{ display: "flex", gap: 2 }}>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={handleRefresh}
          >
            Refresh
          </Button>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => router.push("/dashboard/tunnels/create")}
          >
            Create Tunnel
          </Button>
        </Box>
      </Box>

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Protocol</TableCell>
              <TableCell>Local Port</TableCell>
              <TableCell>Public URL</TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {tunnels.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} align="center">
                  <Typography variant="body1" sx={{ py: 2 }}>
                    No tunnels found. Click "Create Tunnel" to get started.
                  </Typography>
                </TableCell>
              </TableRow>
            ) : (
              tunnels.map((tunnel: Tunnel) => (
                <TableRow key={tunnel.id}>
                  <TableCell>{tunnel.name}</TableCell>
                  <TableCell>{tunnel.protocol.toUpperCase()}</TableCell>
                  <TableCell>{tunnel.localPort}</TableCell>
                  <TableCell>
                    {tunnel.publicUrl ? (
                      <a
                        href={tunnel.publicUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        style={{ textDecoration: "none" }}
                      >
                        {tunnel.publicUrl}
                      </a>
                    ) : (
                      "-"
                    )}
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={tunnel.status}
                      color={getStatusColor(tunnel.status)}
                      size="small"
                    />
                  </TableCell>
                  <TableCell align="right">
                    <IconButton
                      color="error"
                      onClick={() => handleDelete(tunnel.id)}
                      disabled={isDeleting === tunnel.id}
                    >
                      {isDeleting === tunnel.id ? (
                        <CircularProgress size={20} />
                      ) : (
                        <DeleteIcon />
                      )}
                    </IconButton>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  );
}
