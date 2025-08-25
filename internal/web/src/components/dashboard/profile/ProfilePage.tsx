"use client";

import { useActionState } from "react";
import { Box, Paper, Typography, TextField, Button, Grid, Avatar } from "@mui/material";
import { updateProfileAction } from "@/lib/actions/user.actions";
import { User } from "@/lib/actions/user.types";

interface ProfilePageProps {
  initialUser: User;
}

export default function ProfilePage({ initialUser }: ProfilePageProps) {
  const [state, action, loading] = useActionState(updateProfileAction, {
    name: initialUser.name,
    email: initialUser.email,
  });

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
                {state.name.charAt(0) || "U"}
              </Avatar>
              <Typography variant="h6">{state.name}</Typography>
              <Typography color="text.secondary">{state.email}</Typography>
            </Box>
          </Grid>
          <Grid size={{ xs: 12, md: 8 }}>
            <Box component="form" action={action}>
              <TextField
                margin="normal"
                required
                fullWidth
                id="name"
                label="Full Name"
                name="name"
                autoComplete="name"
                defaultValue={state.name}
              />
              <Button type="submit" fullWidth variant="contained" sx={{ mt: 3 }} disabled={loading}>
                {loading ? "Saving..." : "Save Changes"}
              </Button>
            </Box>
          </Grid>
        </Grid>
      </Paper>
    </Box>
  );
}
