import { Typography, Box, Container } from "@mui/material";
import type { Metadata } from "next";
import ContactForm from "@/components/common/ContactForm";

export const metadata: Metadata = {
  title: "Contact | GiraffeCloud",
  description: "Contact GiraffeCloud support and sales.",
};

export default function ContactPage() {
  return (
    <Container maxWidth="sm">
      <Box sx={{ py: 8 }}>
        <Typography variant="h2" component="h1" gutterBottom>
          Contact Us
        </Typography>
        <Typography gutterBottom color="text.secondary">
          Questions about{/* pricing,*/} compliance or technical capabilities? Send us a message.
        </Typography>
        <ContactForm />
      </Box>
    </Container>
  );
}
