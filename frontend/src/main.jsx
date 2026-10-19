import React from "react";
import ReactDOM from "react-dom/client";
import { createBrowserRouter, Link, RouterProvider } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "./queryClient";
import { AuthProvider, RequireAuth } from "./auth";
import { ThemeProvider } from "./theme";
import Layout from "./components/Layout";
import Login from "./pages/Login";
import Archives from "./pages/Archives";
import Book from "./pages/Book";
import Conversation from "./pages/Conversation";
import ShiftArchive from "./pages/ShiftArchive";
import "./styles.css";
import "./brand.css";
import "./conversation.css";

if (import.meta.env.PROD && "serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker
      .register("/sw.js", { updateViaCache: "none" })
      .catch((error) => console.warn("PWA registration failed:", error));
  });
}

const router = createBrowserRouter([
  { path: "/login", element: <Login /> },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <Layout />,
        children: [
          { index: true, element: <Archives /> },
          { path: "/machines/:machineId", element: <Conversation /> },
          {
            path: "/machines/:machineId/shifts/:date/:shiftId",
            element: <ShiftArchive />,
          },
          { path: "/logbooks/:id", element: <Book /> },
        ],
      },
    ],
  },
  {
    path: "*",
    element: (
      <main className="standalone">
        <p className="section-label">404</p>
        <h1>Halaman tidak ditemukan.</h1>
        <Link to="/" className="button button-primary">
          Pilih mesin
        </Link>
      </main>
    ),
  },
]);

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AuthProvider>
          <RouterProvider router={router} />
        </AuthProvider>
      </ThemeProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
