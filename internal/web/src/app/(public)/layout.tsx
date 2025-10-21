import Footer from "@/components/common/Footer";
import { Box, Container } from "@mui/material";

type PublicLayoutProps = {
  children: React.ReactNode;
};

export default function PublicLayout({ children }: PublicLayoutProps) {
  const footerHeight = 113;

  return (
    <main>
      <Container maxWidth="lg">
        <Box sx={{ minHeight: `calc(100vh - ${footerHeight}px)` }}>{children}</Box>
      </Container>
      <Footer />
    </main>
  );
}
