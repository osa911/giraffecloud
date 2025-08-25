"use client";

import React from "react";
import { Box, Container, Paper, Tab, Tabs, Typography } from "@mui/material";
import TokenManagement from "@/components/dashboard/settings/TokenManagement";

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel(props: TabPanelProps) {
  const { children, value, index, ...other } = props;

  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`settings-tabpanel-${index}`}
      aria-labelledby={`settings-tab-${index}`}
      {...other}
    >
      {value === index && <Box sx={{ p: 3 }}>{children}</Box>}
    </div>
  );
}

function a11yProps(index: number) {
  return {
    id: `settings-tab-${index}`,
    "aria-controls": `settings-tabpanel-${index}`,
  };
}

type Props = {};

const SettingsPage = (props: Props) => {
  const [value, setValue] = React.useState(0);

  const handleChange = (event: React.SyntheticEvent, newValue: number) => {
    setValue(newValue);
  };

  return (
    <Container maxWidth="lg" sx={{ mt: 4 }}>
      <Typography variant="h4" gutterBottom>
        Settings
      </Typography>
      <Paper sx={{ width: "100%" }}>
        <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
          <Tabs value={value} onChange={handleChange} aria-label="settings tabs">
            <Tab label="API Tokens" {...a11yProps(0)} />
            {/* Add more tabs here as needed */}
          </Tabs>
        </Box>
        <TabPanel value={value} index={0}>
          <TokenManagement />
        </TabPanel>
        {/* Add more TabPanels here as needed */}
      </Paper>
    </Container>
  );
};

export default SettingsPage;
