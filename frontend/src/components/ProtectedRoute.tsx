import { Navigate, Outlet } from "react-router-dom";
import { getToken } from "../api/client";

export function ProtectedRoute() {
  return getToken() ? <Outlet /> : <Navigate to="/login" replace />;
}
