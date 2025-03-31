"use client";

import { useState, FormEvent, ChangeEvent, use } from "react";
import { useRouter } from "next/navigation";
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  SelectChangeEvent,
} from "@mui/material";
import { api } from "@/services/api";

interface TunnelForm {
  name: string;
  localPort: string;
  protocol: string;
}

export default function CreateTunnel() {
  const router = useRouter();
  const [form, setForm] = useState<TunnelForm>({
    name: "",
    localPort: "",
    protocol: "http",
  });
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setLoading(true);
    try {
      await api.post("/tunnels", {
        ...form,
        localPort: parseInt(form.localPort),
      });
      router.push("/dashboard/tunnels");
    } catch (error) {
      // Error is handled by the API interceptor
    } finally {
      setLoading(false);
    }
  }

  function handleTextChange(e: ChangeEvent<HTMLInputElement>) {
    const { name, value } = e.target;
    setForm((prev) => ({
      ...prev,
      [name]: value,
    }));
  }

  function handleSelectChange(e: SelectChangeEvent) {
    const { name, value } = e.target;
    setForm((prev) => ({
      ...prev,
      [name]: value,
    }));
  }

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Create Tunnel
      </Typography>
      <Paper sx={{ p: 3 }}>
        <Box component="form" onSubmit={handleSubmit}>
          <TextField
            margin="normal"
            required
            fullWidth
            id="name"
            label="Tunnel Name"
            name="name"
            autoComplete="name"
            value={form.name}
            onChange={handleTextChange}
          />
          <TextField
            margin="normal"
            required
            fullWidth
            id="localPort"
            label="Local Port"
            name="localPort"
            type="number"
            autoComplete="off"
            value={form.localPort}
            onChange={handleTextChange}
          />
          <FormControl fullWidth margin="normal">
            <InputLabel id="protocol-label">Protocol</InputLabel>
            <Select
              labelId="protocol-label"
              id="protocol"
              name="protocol"
              value={form.protocol}
              label="Protocol"
              onChange={handleSelectChange}
            >
              <MenuItem value="http">HTTP</MenuItem>
              <MenuItem value="https">HTTPS</MenuItem>
              <MenuItem value="tcp">TCP</MenuItem>
              <MenuItem value="udp">UDP</MenuItem>
            </Select>
          </FormControl>
          <Box sx={{ mt: 3, display: "flex", gap: 2 }}>
            <Button
              type="submit"
              variant="contained"
              disabled={loading}
              sx={{ flex: 1 }}
            >
              {loading ? "Creating..." : "Create Tunnel"}
            </Button>
            <Button
              variant="outlined"
              onClick={() => router.push("/dashboard/tunnels")}
              sx={{ flex: 1 }}
            >
              Cancel
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
