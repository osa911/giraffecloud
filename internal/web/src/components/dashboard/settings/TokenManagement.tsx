"use client";

import React, { useState, useEffect } from "react";
import {
  Box,
  Button,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  Alert,
  Snackbar,
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import { format } from "date-fns";
import { getTokensList, revokeToken, type Token } from "@/api/tokenApi";
import { ApiError } from "@/utils/error";
import TokenDialog from "./TokenDialog";

const TokenManagement: React.FC = () => {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [open, setOpen] = useState(false);
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

  const handleTokenCreated = () => {
    // Refresh the tokens list after a new token is created
    fetchTokens();
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

      <TokenDialog open={open} onClose={handleClose} onSuccess={handleTokenCreated} />

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
