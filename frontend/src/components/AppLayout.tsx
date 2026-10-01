import { Link as RouterLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import AppBar from "@mui/material/AppBar";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import IconButton from "@mui/material/IconButton";
import LogoutIcon from "@mui/icons-material/Logout";
import Container from "@mui/material/Container";
import Box from "@mui/material/Box";
import { clearToken } from "../api/client";

export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const currentTab = location.pathname.startsWith("/search") ? "/search" : "/library";

  const handleLogout = () => {
    clearToken();
    navigate("/login");
  };

  return (
    <Box>
      <AppBar position="static">
        <Toolbar>
          <Typography variant="h6" sx={{ mr: 4 }}>
            Кинотеатр
          </Typography>
          <Tabs value={currentTab} textColor="inherit" indicatorColor="secondary" sx={{ flexGrow: 1 }}>
            <Tab label="Библиотека" value="/library" component={RouterLink} to="/library" />
            <Tab label="Поиск" value="/search" component={RouterLink} to="/search" />
          </Tabs>
          <IconButton color="inherit" onClick={handleLogout} aria-label="выйти">
            <LogoutIcon />
          </IconButton>
        </Toolbar>
      </AppBar>
      <Container sx={{ py: 3 }}>
        <Outlet />
      </Container>
    </Box>
  );
}
