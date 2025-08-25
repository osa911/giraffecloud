"use client";
import { Box, TextField, Button, Stack, Typography } from "@mui/material";
import { useState } from "react";
import toast from "react-hot-toast";

export default function ContactForm() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !message) {
      toast.error("Please provide an email and message.");
      return;
    }
    setLoading(true);
    try {
      // TODO: Wire to backend endpoint or email service
      await new Promise((res) => setTimeout(res, 500));
      toast.success("Message sent. We'll get back to you shortly.");
      setName("");
      setEmail("");
      setMessage("");
    } catch (err) {
      toast.error("Failed to send. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box component="form" onSubmit={onSubmit} noValidate>
      <Stack spacing={2}>
        <TextField label="Name" value={name} onChange={(e) => setName(e.target.value)} />
        <TextField
          label="Email"
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
        <TextField
          label="Message"
          required
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          multiline
          minRows={4}
        />
        <Button type="submit" variant="contained" disabled={loading}>
          {loading ? "Sending..." : "Send"}
        </Button>
        <Typography variant="body2" color="text.secondary">
          We will use your information to respond to your inquiry. See our Privacy Policy for
          details.
        </Typography>
      </Stack>
    </Box>
  );
}
