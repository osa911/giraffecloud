"use client";

import { useState } from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { MoreHorizontal, Plus, Pencil, Trash, ExternalLink, AlertTriangle, RefreshCw } from "lucide-react";
import { format } from "date-fns";
import TunnelDialog from "./TunnelDialog";
import { useTunnels } from "@/hooks/useTunnels";
import { type Tunnel, DnsPropagationStatus } from "@/types/tunnel";
import clientApi from "@/services/apiClient/clientApiClient";
import { toast } from "@/lib/toast";
import Link from "next/link";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

export default function TunnelList() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [selectedTunnel, setSelectedTunnel] = useState<Tunnel | null>(null);
  const [tunnelToDelete, setTunnelToDelete] = useState<Tunnel | null>(null);
  const { tunnels, isLoading, mutate } = useTunnels();
  const [verifyingId, setVerifyingId] = useState<number | null>(null);

  const baseDomain = process.env.NEXT_PUBLIC_BASE_DOMAIN || "giraffecloud.xyz";
  const serverIP = process.env.NEXT_PUBLIC_SERVER_IP || "your-server-ip";

  const isCustomDomain = (domain: string) => {
    return !domain.endsWith(`.${baseDomain}`);
  };

  const handleOpenDialog = (tunnel?: Tunnel) => {
    setSelectedTunnel(tunnel || null);
    setIsDialogOpen(true);
  };

  const handleCloseDialog = () => {
    setIsDialogOpen(false);
    setSelectedTunnel(null);
  };

  const handleToggleActive = async (tunnel: Tunnel) => {
    try {
      if (!tunnel.is_enabled) {
        setVerifyingId(tunnel.id);
      }

      // Optimistic update could be done here, but for simplicity we wait
      await clientApi().put<Tunnel>(`/tunnels/${tunnel.id}`, {
        is_enabled: !tunnel.is_enabled,
      });
      mutate(); // Refresh the tunnels list
      toast.success(`Tunnel ${!tunnel.is_enabled ? "enabled" : "disabled"}`);
    } catch (error: unknown) {
      console.error("Error updating tunnel status:", error);

      // Extract error message from response
      let errorMessage = "Failed to update tunnel status";

      if (error && typeof error === 'object' && 'response' in error) {
        const axiosError = error as { response?: { data?: { error?: string | { message?: string } } } };
        const responseError = axiosError.response?.data?.error;
        if (typeof responseError === 'string') {
          errorMessage = responseError;
        } else if (responseError && typeof responseError === 'object' && 'message' in responseError) {
          errorMessage = responseError.message || errorMessage;
        }
      }

      // Show appropriate error message based on context
      if (!tunnel.is_enabled) {
        // Trying to enable - likely DNS verification failed
        toast.error(errorMessage.includes("DNS") || errorMessage.includes("domain")
          ? errorMessage
          : "DNS verification failed. Please ensure your domain points to the server IP.");
      } else {
        toast.error(errorMessage);
      }
    } finally {
      setVerifyingId(null);
    }
  };

  const handleDelete = (tunnel: Tunnel) => {
    setTunnelToDelete(tunnel);
  };

  const confirmDelete = async () => {
    if (!tunnelToDelete) return;

    try {
      await clientApi().delete<void>(`/tunnels/${tunnelToDelete.id}`);
      mutate(); // Refresh the tunnels list
      toast.success("Tunnel deleted successfully");
    } catch (error) {
      console.error("Error deleting tunnel:", error);
      toast.error("Failed to delete tunnel");
    } finally {
      setTunnelToDelete(null);
    }
  };

  if (isLoading) {
    return <div className="p-8 text-center">Loading tunnels...</div>;
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle className="text-xl font-bold">Tunnels</CardTitle>
          <Button onClick={() => handleOpenDialog()}>
            <Plus className="mr-2 h-4 w-4" /> Create Tunnel
          </Button>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Domain</TableHead>
                  <TableHead>Target Port</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tunnels?.map((tunnel) => (
                  <TableRow key={tunnel.id}>
                    <TableCell className="font-medium">
                      <Link
                        href={`https://${tunnel.domain}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center hover:underline text-primary"
                      >
                        {tunnel.domain}
                        <ExternalLink className="ml-1 h-3 w-3" />
                      </Link>
                    </TableCell>
                    <TableCell>{tunnel.target_port}</TableCell>
                    <TableCell>
                      <div className="flex items-center space-x-2">
                        <Switch
                          checked={tunnel.is_enabled}
                          onCheckedChange={() => handleToggleActive(tunnel)}
                          disabled={verifyingId === tunnel.id}
                        />
                        {isCustomDomain(tunnel.domain) && tunnel.dns_propagation_status === DnsPropagationStatus.PENDING_DNS && (
                          <TooltipProvider>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <div className="flex items-center text-amber-500 cursor-help">
                                  <AlertTriangle className="h-4 w-4 mr-1" />
                                  <span className="text-xs font-medium">Pending DNS</span>
                                </div>
                              </TooltipTrigger>
                              <TooltipContent>
                                <p>Waiting for domain to point to <strong>{serverIP}</strong></p>
                                <p className="text-xs text-muted-foreground mt-1">Click the switch to verify & enable</p>
                              </TooltipContent>
                            </Tooltip>
                          </TooltipProvider>
                        )}
                        {verifyingId === tunnel.id && (
                           <RefreshCw className="h-4 w-4 animate-spin text-muted-foreground" />
                        )}
                      </div>
                    </TableCell>
                    <TableCell>{format(new Date(tunnel.created_at), "PPp")}</TableCell>
                    <TableCell>{format(new Date(tunnel.updated_at), "PPp")}</TableCell>
                    <TableCell className="text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" className="h-8 w-8 p-0">
                            <span className="sr-only">Open menu</span>
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuLabel>Actions</DropdownMenuLabel>
                          <DropdownMenuItem onClick={() => navigator.clipboard.writeText(`https://${tunnel.domain}`)}>
                            Copy URL
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem onClick={() => handleOpenDialog(tunnel)}>
                            <Pencil className="mr-2 h-4 w-4" /> Edit
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => handleDelete(tunnel)} className="text-destructive focus:text-destructive">
                            <Trash className="mr-2 h-4 w-4" /> Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
                {(!tunnels || tunnels.length === 0) && (
                  <TableRow>
                    <TableCell colSpan={6} className="h-24 text-center">
                      No tunnels found. Create your first tunnel to get started.
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <TunnelDialog
        open={isDialogOpen}
        onOpenChange={setIsDialogOpen}
        tunnel={selectedTunnel}
        onSuccess={() => {
          mutate();
        }}
        existingTunnels={tunnels}
      />

      <AlertDialog open={!!tunnelToDelete} onOpenChange={(open) => !open && setTunnelToDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Are you sure?</AlertDialogTitle>
            <AlertDialogDescription>
              This action cannot be undone. This will permanently delete the tunnel
              {tunnelToDelete && <span className="font-semibold text-foreground"> {tunnelToDelete.domain}</span>} and remove your data from our servers.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
