"use client";

import React, { useState, useEffect } from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
  Alert,
  Snackbar,
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import { format } from "date-fns";
import { createToken, getTokensList, revokeToken, type Token } from "@/api/tokenApi";
import { ApiError } from "@/utils/error";

const TokenManagement: React.FC = () => {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [open, setOpen] = useState(false);
  const [newTokenName, setNewTokenName] = useState("");
  const [newTokenValue, setNewTokenValue] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchTokens = async () => {
    try {
      setLoading(true);
      const data = await getTokensList();
      setTokens(data);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to fetch tokens");
      console.error("Error fetching tokens:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTokens();
  }, []);

  const handleCreateToken = async () => {
    try {
      setLoading(true);
      const data = await createToken(newTokenName);
      setNewTokenValue(data.token);
      setTokens([
        ...tokens,
        {
          id: data.id,
          name: data.name,
          created_at: data.created_at,
          last_used_at: data.created_at,
          expires_at: data.expires_at,
        },
      ]);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create token");
      console.error("Error creating token:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleRevokeToken = async (id: string) => {
    try {
      setLoading(true);
      await revokeToken(id);
      setTokens(tokens.filter((token) => token.id !== id));
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to revoke token");
      console.error("Error revoking token:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setOpen(false);
    setNewTokenName("");
    setNewTokenValue(null);
  };

  const handleErrorClose = () => {
    setError(null);
  };

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", mb: 3 }}>
        <Typography variant="h5">API Tokens</Typography>
        <Button variant="contained" onClick={() => setOpen(true)} disabled={loading}>
          Create New Token
        </Button>
      </Box>

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Created</TableCell>
              <TableCell>Last Used</TableCell>
              <TableCell>Expires</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {tokens.map((token) => (
              <TableRow key={token.id}>
                <TableCell>{token.name}</TableCell>
                <TableCell>{format(new Date(token.created_at), "PPp")}</TableCell>
                <TableCell>{format(new Date(token.last_used_at), "PPp")}</TableCell>
                <TableCell>{format(new Date(token.expires_at), "PPp")}</TableCell>
                <TableCell>
                  <IconButton
                    onClick={() => handleRevokeToken(token.id)}
                    color="error"
                    size="small"
                    disabled={loading}
                  >
                    <DeleteIcon />
                  </IconButton>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
        <DialogTitle>{newTokenValue ? "Save Your Token" : "Create New Token"}</DialogTitle>
        <DialogContent>
          {newTokenValue ? (
            <Box sx={{ mt: 2 }}>
              <Typography variant="body2" color="error" gutterBottom>
                Make sure to copy your token now. You won't be able to see it again!
              </Typography>
              <TextField
                fullWidth
                value={newTokenValue}
                variant="outlined"
                InputProps={{
                  readOnly: true,
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
              Create
            </Button>
          )}
        </DialogActions>
      </Dialog>

      <Snackbar
        open={!!error}
        autoHideDuration={6000}
        onClose={handleErrorClose}
        anchorOrigin={{ vertical: "bottom", horizontal: "center" }}
      >
        <Alert onClose={handleErrorClose} severity="error" sx={{ width: "100%" }}>
          {error}
        </Alert>
      </Snackbar>
    </Box>
  );
};

export default TokenManagement;
