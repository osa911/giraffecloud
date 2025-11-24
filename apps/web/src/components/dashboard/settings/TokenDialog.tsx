"use client";

import React, { useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Typography,
  Box,
  InputAdornment,
  IconButton,
  Tooltip,
} from "@mui/material";
import { ContentCopy as ContentCopyIcon, Check as CheckIcon } from "@mui/icons-material";
import toast from "react-hot-toast";
import { createToken } from "@/api/tokenApi";

interface TokenDialogProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (tokenValue: string) => void;
}

export default function TokenDialog({ open, onClose, onSuccess }: TokenDialogProps) {
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
    setLoading(true);
    try {
      const data = await createToken(newTokenName);
      setNewTokenValue(data.token);
      toast.success("Token created successfully!");
      onSuccess(data.token);
    } catch (error) {
      console.error("Error creating token:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setNewTokenName("");
    setNewTokenValue(null);
    setCopied(false);
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{newTokenValue ? "Save Your Token" : "Create New Token"}</DialogTitle>
      <DialogContent>
        {newTokenValue ? (
          <Box sx={{ mt: 2 }}>
            <Typography variant="body2" color="error" gutterBottom>
              Make sure to copy your token now. You won&apos;t be able to see it again!
            </Typography>
            <TextField
              fullWidth
              value={newTokenValue}
              variant="outlined"
              InputProps={{
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <Tooltip title={copied ? "Copied!" : "Copy to clipboard"}>
                      <IconButton onClick={handleCopyToken} edge="end" color={copied ? "success" : "default"}>
                        {copied ? <CheckIcon /> : <ContentCopyIcon />}
                      </IconButton>
                    </Tooltip>
                  </InputAdornment>
                ),
              }}
            />
          </Box>
        ) : (
          <TextField
            autoFocus
            margin="dense"
            label="Token Name"
            fullWidth
            variant="outlined"
            value={newTokenName}
            onChange={(e) => setNewTokenName(e.target.value)}
            disabled={loading}
            helperText="Give your token a descriptive name (e.g., 'My Laptop', 'Production Server')"
          />
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={loading}>
          {newTokenValue ? "Close" : "Cancel"}
        </Button>
        {!newTokenValue && (
          <Button
            onClick={handleCreateToken}
            variant="contained"
            disabled={loading || !newTokenName.trim()}
          >
            {loading ? "Creating..." : "Create"}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}

