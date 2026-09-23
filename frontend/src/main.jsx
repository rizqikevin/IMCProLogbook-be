import React from "react";
import ReactDOM from "react-dom/client";
import { createBrowserRouter, Link, RouterProvider } from "react-router-dom";
import { AuthProvider, RequireAuth } from "./auth";
import Layout from "./components/Layout";
import Login from "./pages/Login";
import Archives from "./pages/Archives";
import Book from "./pages/Book";
import Capture from "./pages/Capture";
import "./styles.css";

const router = createBrowserRouter([
  { path: "/login", element: <Login /> },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <Layout />,
        children: [
          { index: true, element: <Archives /> },
          { path: "/new", element: <Capture /> },
          { path: "/logbooks/:id", element: <Book /> },
          { path: "/logbooks/:id/add", element: <Capture /> },
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
          Kembali ke arsip
        </Link>
      </main>
    ),
  },
]);

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <AuthProvider>
      <RouterProvider router={router} />
    </AuthProvider>
  </React.StrictMode>,
);
