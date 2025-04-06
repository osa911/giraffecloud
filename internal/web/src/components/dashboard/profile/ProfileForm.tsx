"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  Grid,
  Avatar,
} from "@mui/material";
import { useAuth, User } from "@/contexts/AuthProvider";
import clientApi from "@/services/api/clientApiClient";
import toast from "react-hot-toast";
import { handleLoginSuccess } from "@/lib/actions";

interface ProfileFormProps {
  initialUser: User;
}

export default function ProfileForm({ initialUser }: ProfileFormProps) {
  const { updateUser } = useAuth();
  const [name, setName] = useState(initialUser.name || "");
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const updatedUser = await clientApi().put<User>("/user/profile", {
        name,
      });

      // Update auth context
      updateUser(updatedUser);

      // Update cookie data to keep it in sync
      await handleLoginSuccess(updatedUser);

      toast.success("Profile updated successfully");
      router.refresh(); // Refresh server components
    } catch (error) {
      toast.error("Failed to update profile");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        Profile
      </Typography>
      <Paper sx={{ p: 3 }}>
        <Grid container spacing={3}>
          <Grid size={{ xs: 12, md: 4 }}>
            <Box sx={{ textAlign: "center" }}>
              <Avatar
                sx={{
                  width: 120,
                  height: 120,
                  mx: "auto",
                  mb: 2,
                }}
              >
                {initialUser?.name?.charAt(0) || "U"}
              </Avatar>
              <Typography variant="h6">{initialUser.name}</Typography>
              <Typography color="text.secondary">
                {initialUser.email}
              </Typography>
            </Box>
          </Grid>
          <Grid size={{ xs: 12, md: 8 }}>
            <Box component="form" onSubmit={handleSubmit}>
              <TextField
                margin="normal"
                required
                fullWidth
                id="name"
                label="Full Name"
                name="name"
                autoComplete="name"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                sx={{ mt: 3 }}
                disabled={loading}
              >
                {loading ? "Saving..." : "Save Changes"}
              </Button>
            </Box>
          </Grid>
        </Grid>
      </Paper>
    </Box>
  );
}
