using System.Diagnostics;
using System.Net.Sockets;

class DistributedClient
{
	static void Main(string[] args)
	{
		string[] servers = {
			"localhost:5001",
			"localhost:5002",
			"localhost:5003",
		};

		Console.Write("Enter N: ");
		int N = int.Parse(Console.ReadLine());

		Console.WriteLine($"\n=== Distributed Computing with {servers.Length} servers ===\n");

		var sw = Stopwatch.StartNew();
		int chunk = N / servers.Length;
		List<Task<long>> tasks = new List<Task<long>>();
		for (int i = 0; i < servers.Length; i++)
		{
			int start = i * chunk + 1;
			int end = (i == servers.Length - 1) ? N : (i + 1) * chunk;
			string server = servers[i];

			tasks.Add(SendTaskToServer(server, "sum", start, end));
		}
		Task.WaitAll(tasks.ToArray());
		long sum = tasks.Sum(t => t.Result);

		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	static async Task<long> SendTaskToServer(string serverAddr, string mode, int start, int end)
	{
		string[] parts = serverAddr.Split(':');
		string host = parts[0];
		int port = int.Parse(parts[1]);

		TcpClient client = new TcpClient();
		client.Connect(host, port);
		NetworkStream stream = client.GetStream();

		string request = $"{mode}:{start}:{end}";
		byte[] data = System.Text.Encoding.UTF8.GetBytes(request);
		await stream.WriteAsync(data, 0, data.Length);

		byte[] buffer = new byte[1024];
		int bytes = await stream.ReadAsync(buffer, 0, buffer.Length);
		string response = System.Text.Encoding.UTF8.GetString(buffer, 0, bytes);
		client.Close();

		return long.Parse(response);
	}

	static bool IsPrime(int n)
	{
		if (n <= 1) return false;
		if (n <= 3) return true;
		if (n % 2 == 0 || n % 3 == 0) return false;
		for (int i = 5; i * i <= n; i += 6)
			if (n % i == 0 || n % (i + 2) == 0)
				return false;
		return true;
	}
}