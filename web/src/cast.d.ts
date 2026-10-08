// The part of the Cast Web Receiver framework (CAF v3) that Daycast uses.
// The framework is loaded as a global script from gstatic, see
// https://developers.google.com/cast/docs/reference/web_receiver

declare namespace cast.framework {
  class CastReceiverOptions {
    disableIdleTimeout?: boolean;
    skipPlayersLoad?: boolean;
    customNamespaces?: Record<string, system.MessageType>;
    statusText?: string;
  }

  interface CustomMessageEvent {
    senderId: string;
    data: unknown;
  }

  class CastReceiverContext {
    static getInstance(): CastReceiverContext;
    start(options?: CastReceiverOptions): CastReceiverContext;
    addCustomMessageListener(namespace: string, listener: (event: CustomMessageEvent) => void): void;
    sendCustomMessage(namespace: string, senderId: string | undefined, data: unknown): void;
    addEventListener(type: system.EventType, handler: (event: system.SenderEvent) => void): void;
    getSenders(): unknown[];
  }

  namespace system {
    enum MessageType {
      STRING = 'STRING',
      JSON = 'JSON',
    }
    enum EventType {
      SENDER_CONNECTED = 'senderconnected',
      SENDER_DISCONNECTED = 'senderdisconnected',
    }
    interface SenderEvent {
      senderId: string;
    }
  }
}
