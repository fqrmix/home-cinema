import { useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import Box from "@mui/material/Box";
import TextField from "@mui/material/TextField";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";
import Alert from "@mui/material/Alert";
import { useLogin } from "../api/auth";

export function LoginPage() {
  const [password, setPassword] = useState("");
  const navigate = useNavigate();
  const login = useLogin();

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    login.mutate(password, {
      onSuccess: () => navigate("/library"),
    });
  };

  return (
    <Box display="flex" alignItems="center" justifyContent="center" minHeight="100vh">
      <Box component="form" onSubmit={handleSubmit} sx={{ width: 320 }}>
        <Typography variant="h5" sx={{ mb: 2 }}>
          Домашний кинотеатр
        </Typography>
        <TextField
          fullWidth
          type="password"
          label="Пароль"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          sx={{ mb: 2 }}
          autoFocus
        />
        {login.isError ? (
          <Alert severity="error" sx={{ mb: 2 }}>
            Неверный пароль
          </Alert>
        ) : null}
        <Button type="submit" fullWidth variant="contained" disabled={login.isPending}>
          Войти
        </Button>
      </Box>
    </Box>
  );
}
