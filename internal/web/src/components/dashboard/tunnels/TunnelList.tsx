"use client";

import { useState } from "react";
import {
  Box,
  Button,
  Card,
  IconButton,
  Stack,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from "@mui/material";
import { Edit as EditIcon, Delete as DeleteIcon } from "@mui/icons-material";
import { format } from "date-fns";
import toast from "react-hot-toast";
import TunnelDialog from "./TunnelDialog";
import { useTunnels } from "@/hooks/useTunnels";
import type { Tunnel } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";

export default function TunnelList() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [selectedTunnel, setSelectedTunnel] = useState<Tunnel | null>(null);
  const { tunnels, isLoading, mutate } = useTunnels();

  const handleOpenDialog = (tunnel?: Tunnel) => {
    setSelectedTunnel(tunnel || null);
    setIsDialogOpen(true);
  };

  const handleCloseDialog = () => {
    setIsDialogOpen(false);
    setSelectedTunnel(null);
  };

  const handleToggleActive = async (tunnel: Tunnel) => {
    try {
      await clientApi().put<Tunnel>(`/tunnels/${tunnel.id}`, {
        is_active: !tunnel.is_active,
      });

      mutate(); // Refresh the tunnels list
    } catch (error) {
      // Error handling is done by clientApi
      console.error("Error updating tunnel status:", error);
    }
  };

  const handleDelete = async (tunnel: Tunnel) => {
    if (!window.confirm("Are you sure you want to delete this tunnel?")) {
      return;
    }

    try {
      await clientApi().delete<void>(`/tunnels/${tunnel.id}`);
      mutate(); // Refresh the tunnels list
    } catch (error) {
      // Error handling is done by clientApi
      console.error("Error deleting tunnel:", error);
    }
  };

  if (isLoading) {
    return (
      <Box sx={{ p: 3 }}>
        <Typography>Loading tunnels...</Typography>
      </Box>
    );
  }

  return (
    <>
      <Card>
        <Box sx={{ p: 3 }}>
          <Stack
            direction="row"
            justifyContent="space-between"
            alignItems="center"
            spacing={2}
          >
            <Typography variant="h5">Tunnels</Typography>
            <Button
              variant="contained"
              color="primary"
              onClick={() => handleOpenDialog()}
            >
              Create Tunnel
            </Button>
          </Stack>
        </Box>

        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Domain</TableCell>
                <TableCell>Target Port</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Created</TableCell>
                <TableCell>Updated</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {tunnels?.map((tunnel) => (
                <TableRow key={tunnel.id}>
                  <TableCell>{tunnel.domain}</TableCell>
                  <TableCell>{tunnel.target_port}</TableCell>
                  <TableCell>
                    <Switch
                      checked={tunnel.is_active}
                      onChange={() => handleToggleActive(tunnel)}
                      color="primary"
                    />
                  </TableCell>
                  <TableCell>
                    {format(new Date(tunnel.created_at), "PPp")}
                  </TableCell>
                  <TableCell>
                    {format(new Date(tunnel.updated_at), "PPp")}
                  </TableCell>
                  <TableCell align="right">
                    <IconButton
                      size="small"
                      onClick={() => handleOpenDialog(tunnel)}
                    >
                      <EditIcon />
                    </IconButton>
                    <IconButton
                      size="small"
                      onClick={() => handleDelete(tunnel)}
                    >
                      <DeleteIcon />
                    </IconButton>
                  </TableCell>
                </TableRow>
              ))}
              {(!tunnels || tunnels.length === 0) && (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <Typography color="text.secondary">
                      No tunnels found. Create your first tunnel to get started.
                    </Typography>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Card>

      <TunnelDialog
        open={isDialogOpen}
        onClose={handleCloseDialog}
        tunnel={selectedTunnel}
        onSuccess={() => {
          handleCloseDialog();
          mutate();
        }}
      />
    </>
  );
}
