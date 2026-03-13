using System.Net;
using System.Net.Sockets;

class DistributedServer
{
	static void Main(string[] args)
	{
		int port = args.Length > 0 ? int.Parse(args[0]) : 5000;

		// create a TCP listener on the specified port
		TcpListener listener = new(IPAddress.Any, port);
		listener.Start();
		Console.WriteLine($"Server started on port {port}");

		while (true)
		{
			TcpClient client = listener.AcceptTcpClient();
			Console.WriteLine("Client connected");
			HandleClient(client);
		}
	}

	static void HandleClient(TcpClient client)
	{
		NetworkStream stream = client.GetStream();
		byte[] buffer = new byte[1024];
		int bytes = stream.Read(buffer, 0, buffer.Length);
		string request = System.Text.Encoding.UTF8.GetString(buffer, 0, bytes);

		// format: "sum:start:end or prime:start:end"
		string[] parts = request.Split(':');
		string operation = parts[0];
		int start = int.Parse(parts[1]);
		int end = int.Parse(parts[2]);

		long result = 0;
		if (operation == "sum")
		{
			for (int i = start; i <= end; i++)
			{
				result += i;
			}
		}
		else if (operation == "prime")
		{
			for (int i = start; i <= end; i++)
			{
				if (IsPrime(i))
				{
					result++;
				}
			}
		}

		byte[] response = System.Text.Encoding.UTF8.GetBytes(result.ToString());
		stream.Write(response, 0, response.Length);
		client.Close();
	}

	static bool IsPrime(int number)
	{
		if (number <= 1) return false;
		if (number == 2) return true;
		if (number % 2 == 0) return false;

		for (int i = 3; i <= Math.Sqrt(number); i += 2)
		{
			if (number % i == 0) return false;
		}
		return true;
	}
}