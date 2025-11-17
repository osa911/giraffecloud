"use client";

import { useState, useEffect } from "react";
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
  ToggleButtonGroup,
  ToggleButton,
  Alert,
  CircularProgress,
  Box,
  Typography,
  Chip,
} from "@mui/material";
import { AutoAwesome, Language } from "@mui/icons-material";
import toast from "react-hot-toast";
import type { Tunnel, TunnelFormData, TunnelCreateResponse } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";
import { getFreeSubdomain } from "@/hooks/useTunnels";

interface TunnelDialogProps {
  open: boolean;
  onClose: () => void;
  tunnel?: Tunnel | null;
  onSuccess: () => void;
}

type DomainType = "free" | "custom";

export default function TunnelDialog({ open, onClose, tunnel, onSuccess }: TunnelDialogProps) {
  const [domainType, setDomainType] = useState<DomainType>("free");
  const [freeSubdomain, setFreeSubdomain] = useState<string>("");
  const [freeSubdomainAvailable, setFreeSubdomainAvailable] = useState<boolean>(true);
  const [loadingFreeSubdomain, setLoadingFreeSubdomain] = useState(false);
  const [freeSubdomainError, setFreeSubdomainError] = useState<string>("");

  const [formData, setFormData] = useState<TunnelFormData>(() => ({
    domain: tunnel?.domain || "",
    target_port: tunnel?.target_port || 80,
    is_active: tunnel?.is_active ?? true,
  }));

  const [isSubmitting, setIsSubmitting] = useState(false);

  // Reset state when dialog opens
  useEffect(() => {
    if (open) {
      if (tunnel) {
        // Editing existing tunnel - populate form with tunnel data
        setFormData({
          domain: tunnel.domain,
          target_port: tunnel.target_port,
          is_active: tunnel.is_active,
        });
      } else {
        // Creating new tunnel - reset to initial state
        setDomainType("free");
        setFreeSubdomain("");
        setFreeSubdomainAvailable(true);
        setFreeSubdomainError("");
        setFormData({
          domain: "",
          target_port: 80,
          is_active: true,
        });
        // Load free subdomain
        loadFreeSubdomain();
      }
    }
  }, [open, tunnel]);

  const loadFreeSubdomain = async () => {
    setLoadingFreeSubdomain(true);
    setFreeSubdomainError("");
    try {
      const response = await getFreeSubdomain();
      setFreeSubdomain(response.domain);
      setFreeSubdomainAvailable(response.available);

      // If subdomain is not available, switch to custom domain
      if (!response.available) {
        setDomainType("custom");
        setFormData((prev) => ({ ...prev, domain: "" }));
      } else {
        setFormData((prev) => ({ ...prev, domain: response.domain }));
      }
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : "Failed to load free subdomain";
      setFreeSubdomainError(errorMessage);
    } finally {
      setLoadingFreeSubdomain(false);
    }
  };

  const handleDomainTypeChange = (
    _event: React.MouseEvent<HTMLElement>,
    newType: DomainType | null,
  ) => {
    if (newType !== null) {
      setDomainType(newType);
      if (newType === "free" && freeSubdomain) {
        setFormData({ ...formData, domain: freeSubdomain });
      } else if (newType === "custom") {
        setFormData({ ...formData, domain: "" });
      }
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);

    try {
      if (tunnel) {
        await clientApi().put<Tunnel>(`/tunnels/${tunnel.id}`, formData);
        toast.success("Tunnel updated successfully");
      } else {
        const response = await clientApi().post<TunnelCreateResponse>("/tunnels", formData);
        toast.success("Tunnel created successfully");

        // Show token to user (they need it for CLI)
        if (response.token) {
          toast.success(`Token: ${response.token.substring(0, 20)}... (saved to clipboard)`, {
            duration: 5000,
          });
          // Copy token to clipboard
          navigator.clipboard.writeText(response.token);
        }
      }

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
        <DialogTitle>{tunnel ? "Edit Tunnel" : "Create New Tunnel"}</DialogTitle>
        <DialogContent>
          <Stack spacing={3} sx={{ mt: 2 }}>
            {/* Domain Type Selector - only for new tunnels */}
            {!tunnel && (
              <Box>
                <Typography variant="body2" sx={{ mb: 1, fontWeight: 500 }}>
                  Domain Type
                </Typography>
                <ToggleButtonGroup
                  value={domainType}
                  exclusive
                  onChange={handleDomainTypeChange}
                  fullWidth
                  size="small"
                >
                  <ToggleButton
                    value="free"
                    aria-label="free subdomain"
                    disabled={!freeSubdomainAvailable}
                  >
                    <AutoAwesome sx={{ mr: 1, fontSize: 18 }} />
                    Free Subdomain
                  </ToggleButton>
                  <ToggleButton value="custom" aria-label="custom domain">
                    <Language sx={{ mr: 1, fontSize: 18 }} />
                    Custom Domain
                  </ToggleButton>
                </ToggleButtonGroup>
                {!freeSubdomainAvailable && freeSubdomain && (
                  <Alert severity="info" sx={{ mt: 2 }}>
                    You already have a free subdomain: <strong>{freeSubdomain}</strong>
                    <br />
                    To create additional tunnels, please use a custom domain.
                  </Alert>
                )}
              </Box>
            )}

            {/* Free Subdomain Display */}
            {!tunnel && domainType === "free" && (
              <Box>
                {loadingFreeSubdomain ? (
                  <Box display="flex" alignItems="center" gap={2}>
                    <CircularProgress size={20} />
                    <Typography variant="body2" color="text.secondary">
                      Loading your free subdomain...
                    </Typography>
                  </Box>
                ) : freeSubdomainError ? (
                  <Alert severity="error" sx={{ mb: 2 }}>
                    {freeSubdomainError}
                  </Alert>
                ) : freeSubdomain ? (
                  <Box>
                    <Typography variant="body2" sx={{ mb: 1, color: "text.secondary" }}>
                      Your free subdomain:
                    </Typography>
                    <Chip
                      label={freeSubdomain}
                      color="primary"
                      variant="outlined"
                      sx={{ fontFamily: "monospace", fontSize: "0.9rem" }}
                    />
                    <Typography
                      variant="caption"
                      display="block"
                      sx={{ mt: 1, color: "text.secondary" }}
                    >
                      This subdomain is uniquely generated for you and will always be the same.
                    </Typography>
                  </Box>
                ) : null}
              </Box>
            )}

            {/* Custom Domain Input */}
            {(tunnel || domainType === "custom") && (
              <TextField
                label="Domain"
                fullWidth
                required
                value={formData.domain}
                onChange={(e) => setFormData({ ...formData, domain: e.target.value })}
                placeholder="example.com"
                helperText={
                  tunnel ? "Domain cannot be changed after creation" : "Enter your custom domain"
                }
                disabled={!!tunnel}
                InputProps={{
                  readOnly: !!tunnel,
                }}
              />
            )}

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
              helperText="Port on your local machine to forward traffic to"
            />

            {tunnel && (
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
            )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            type="submit"
            variant="contained"
            disabled={isSubmitting || (domainType === "free" && !freeSubdomain && !tunnel)}
          >
            {isSubmitting ? "Saving..." : tunnel ? "Update" : "Create"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
