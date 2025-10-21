"use client";

import { useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import {
  AppBar,
  Box,
  CssBaseline,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
  Menu,
  MenuItem,
  Avatar,
} from "@mui/material";
import {
  Menu as MenuIcon,
  Dashboard as DashboardIcon,
  Person as ProfileIcon,
  Settings as SettingsIcon,
  SwapHoriz as TunnelIcon,
} from "@mui/icons-material";
import { useAuth } from "@/contexts/AuthProvider";
import Link from "@/components/common/Link";
import Footer from "@/components/common/Footer";
import { User } from "@/lib/actions/user.types";
import { ROUTES } from "@/constants/routes";
// import Image from "next/image";
const drawerWidth = 240;

const menuItems = [
  { text: "Dashboard", icon: <DashboardIcon />, path: ROUTES.DASHBOARD.HOME },
  { text: "Tunnels", icon: <TunnelIcon />, path: ROUTES.DASHBOARD.TUNNELS },
  { text: "Profile", icon: <ProfileIcon />, path: ROUTES.DASHBOARD.PROFILE },
  { text: "Settings", icon: <SettingsIcon />, path: ROUTES.DASHBOARD.SETTINGS },
];

interface DashboardLayoutClientProps {
  children: React.ReactNode;
  user: User;
}

export default function DashboardLayoutClient({ children, user }: DashboardLayoutClientProps) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const { logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  const handleMenuOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  const drawer = (
    <div>
      <Toolbar>
        {/* <Image
          src="/giraffecloud.png"
          alt="GiraffeCloud"
          width={48}
          height={48}
        /> */}
        GiraffeCloud
      </Toolbar>
      <List>
        {menuItems.map((item) => (
          <Link
            key={item.text}
            href={item.path}
            style={{ textDecoration: "none", color: "inherit" }}
          >
            <ListItem disablePadding>
              <ListItemButton
                selected={pathname === item.path}
                sx={{
                  "&.Mui-selected": {
                    backgroundColor: "action.selected",
                  },
                  "&:hover": {
                    backgroundColor: "action.hover",
                  },
                }}
              >
                <ListItemIcon>{item.icon}</ListItemIcon>
                <ListItemText primary={item.text} />
              </ListItemButton>
            </ListItem>
          </Link>
        ))}
      </List>
    </div>
  );

  return (
    <Box sx={{ display: "flex" }}>
      <CssBaseline />
      <AppBar
        position="fixed"
        sx={{
          width: { sm: `calc(100% - ${drawerWidth}px)` },
          ml: { sm: `${drawerWidth}px` },
        }}
      >
        <Toolbar>
          <IconButton
            color="inherit"
            edge="start"
            onClick={handleDrawerToggle}
            sx={{ mr: 2, display: { sm: "none" } }}
          >
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            GiraffeCloud
          </Typography>
          <IconButton
            onClick={handleMenuOpen}
            size="small"
            sx={{ ml: 2 }}
            aria-controls="user-menu"
            aria-haspopup="true"
          >
            <Avatar sx={{ width: 32, height: 32 }}>{user?.name?.charAt(0) || "U"}</Avatar>
          </IconButton>
          <Menu
            id="user-menu"
            anchorEl={anchorEl}
            open={Boolean(anchorEl)}
            onClose={handleMenuClose}
            onClick={handleMenuClose}
          >
            <MenuItem onClick={() => router.push("/dashboard/profile")}>Profile</MenuItem>
            <MenuItem onClick={logout}>Logout</MenuItem>
          </Menu>
        </Toolbar>
      </AppBar>
      <Box component="nav" sx={{ width: { sm: drawerWidth }, flexShrink: { sm: 0 } }}>
        <Drawer
          variant="temporary"
          open={mobileOpen}
          onClose={handleDrawerToggle}
          ModalProps={{
            keepMounted: true,
          }}
          sx={{
            display: { xs: "block", sm: "none" },
            "& .MuiDrawer-paper": {
              boxSizing: "border-box",
              width: drawerWidth,
            },
          }}
        >
          {drawer}
        </Drawer>
        <Drawer
          variant="permanent"
          sx={{
            display: { xs: "none", sm: "block" },
            "& .MuiDrawer-paper": {
              boxSizing: "border-box",
              width: drawerWidth,
            },
          }}
          open
        >
          {drawer}
        </Drawer>
      </Box>
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          p: 3,
          pb: 0,
          width: { sm: `calc(100% - ${drawerWidth}px)` },
          display: "flex",
          flexDirection: "column",
          minHeight: "100vh",
        }}
      >
        <Toolbar />
        <Box sx={{ flexGrow: 1 }}>{children}</Box>
        <Footer />
      </Box>
    </Box>
  );
}
