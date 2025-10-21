import Footer from "@/components/common/Footer";
import { Box, Container } from "@mui/material";

type PublicLayoutProps = {
  children: React.ReactNode;
};

export default function PublicLayout({ children }: PublicLayoutProps) {
  const FOOTER_HEIGHT = 97;

  return (
    <main>
      <Container maxWidth="lg">
        <Box
          sx={{
            minHeight: `calc(100vh - ${FOOTER_HEIGHT}px)`,
            display: "flex",
          }}
        >
          {children}
        </Box>
      </Container>
      <Footer />
    </main>
  );
}
