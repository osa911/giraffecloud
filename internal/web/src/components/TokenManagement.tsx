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
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import { format } from "date-fns";

interface Token {
  id: string;
  name: string;
  created_at: string;
  last_used_at: string;
  expires_at: string;
}

interface CreateTokenResponse {
  token: string;
  id: string;
  name: string;
  created_at: string;
  expires_at: string;
}

const TokenManagement: React.FC = () => {
  const [tokens, setTokens] = useState<Token[]>([]);
  const [open, setOpen] = useState(false);
  const [newTokenName, setNewTokenName] = useState("");
  const [newTokenValue, setNewTokenValue] = useState<string | null>(null);

  const fetchTokens = async () => {
    try {
      const response = await fetch("/api/v1/tokens");
      if (!response.ok) throw new Error("Failed to fetch tokens");
      const data = await response.json();
      setTokens(data);
    } catch (error) {
      console.error("Error fetching tokens:", error);
    }
  };

  useEffect(() => {
    fetchTokens();
  }, []);

  const handleCreateToken = async () => {
    try {
      const response = await fetch("/api/v1/tokens", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ name: newTokenName }),
      });

      if (!response.ok) throw new Error("Failed to create token");
      const data: CreateTokenResponse = await response.json();
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
    } catch (error) {
      console.error("Error creating token:", error);
    }
  };

  const handleRevokeToken = async (id: string) => {
    try {
      const response = await fetch(`/api/v1/tokens/${id}`, {
        method: "DELETE",
      });

      if (!response.ok) throw new Error("Failed to revoke token");
      setTokens(tokens.filter((token) => token.id !== id));
    } catch (error) {
      console.error("Error revoking token:", error);
    }
  };

  const handleClose = () => {
    setOpen(false);
    setNewTokenName("");
    setNewTokenValue(null);
  };

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", mb: 3 }}>
        <Typography variant="h5">API Tokens</Typography>
        <Button variant="contained" onClick={() => setOpen(true)}>
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
                <TableCell>
                  {format(new Date(token.created_at), "PPp")}
                </TableCell>
                <TableCell>
                  {format(new Date(token.last_used_at), "PPp")}
                </TableCell>
                <TableCell>
                  {format(new Date(token.expires_at), "PPp")}
                </TableCell>
                <TableCell>
                  <IconButton
                    onClick={() => handleRevokeToken(token.id)}
                    color="error"
                    size="small"
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
        <DialogTitle>
          {newTokenValue ? "Save Your Token" : "Create New Token"}
        </DialogTitle>
        <DialogContent>
          {newTokenValue ? (
            <Box sx={{ mt: 2 }}>
              <Typography variant="body2" color="error" gutterBottom>
                Make sure to copy your token now. You won't be able to see it
                again!
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
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose}>
            {newTokenValue ? "Close" : "Cancel"}
          </Button>
          {!newTokenValue && (
            <Button onClick={handleCreateToken} variant="contained">
              Create
            </Button>
          )}
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default TokenManagement;
