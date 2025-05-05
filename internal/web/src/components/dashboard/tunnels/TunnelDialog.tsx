"use client";

import { useState } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Switch,
  TextField,
  FormControlLabel,
} from "@mui/material";
import toast from "react-hot-toast";
import type { Tunnel, TunnelFormData } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";

interface TunnelDialogProps {
  open: boolean;
  onClose: () => void;
  tunnel?: Tunnel | null;
  onSuccess: () => void;
}

export default function TunnelDialog({
  open,
  onClose,
  tunnel,
  onSuccess,
}: TunnelDialogProps) {
  const [formData, setFormData] = useState<TunnelFormData>(() => ({
    domain: tunnel?.domain || "",
    target_port: tunnel?.target_port || 80,
    is_active: tunnel?.is_active ?? true,
  }));

  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);

    try {
      if (tunnel) {
        await clientApi().put<Tunnel>(`/tunnels/${tunnel.id}`, formData);
      } else {
        await clientApi().post<Tunnel>("/tunnels", formData);
      }

      toast.success(`Tunnel ${tunnel ? "updated" : "created"} successfully`);
      onSuccess();
      onClose();
    } catch (error) {
      // Error handling is done by clientApi
      console.error("Error saving tunnel:", error);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <form onSubmit={handleSubmit}>
        <DialogTitle>
          {tunnel ? "Edit Tunnel" : "Create New Tunnel"}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={3} sx={{ mt: 2 }}>
            <TextField
              label="Domain"
              fullWidth
              required
              value={formData.domain}
              onChange={(e) =>
                setFormData({ ...formData, domain: e.target.value })
              }
              placeholder="example.giraffecloud.dev"
            />
            <TextField
              label="Target Port"
              type="number"
              fullWidth
              required
              value={formData.target_port}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  target_port: parseInt(e.target.value) || 80,
                })
              }
              inputProps={{ min: 1, max: 65535 }}
            />
            <FormControlLabel
              control={
                <Switch
                  checked={formData.is_active}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      is_active: e.target.checked,
                    })
                  }
                />
              }
              label="Active"
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="contained" disabled={isSubmitting}>
            {isSubmitting ? "Saving..." : tunnel ? "Update" : "Create"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
