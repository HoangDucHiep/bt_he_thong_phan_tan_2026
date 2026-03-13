using System.Diagnostics;

class Program
{
	static void Main(string[] args)
	{
		// Worker mode for process pool - sum
		if (args.Length > 0 && args[0] == "worker")
		{
			int start = int.Parse(args[1]);
			int end = int.Parse(args[2]);
			long sum = 0;
			for (int i = start; i <= end; i++)
			{
				sum += i;
			}
			Console.WriteLine(sum);
			return;
		}

		// Worker mode for process pool - prime sum
		if (args.Length > 0 && args[0] == "prime-worker")
		{
			int start = int.Parse(args[1]);
			int end = int.Parse(args[2]);
			long sum = 0;
			for (int i = start; i <= end; i++)
			{
				if (IsPrime(i))
				{
					sum += i;
				}
			}
			Console.WriteLine(sum);
			return;
		}

		// Main menu
		while (true)
		{
			Console.WriteLine("\n=== MENU ===");
			Console.WriteLine("1. Calculate sum of numbers (a, b, d, e)");
			Console.WriteLine("2. Calculate sum of primes (c)");
			Console.WriteLine("3. Exit");
			Console.Write("Select: ");

			string choice = Console.ReadLine();

			if (choice == "1")
			{
				RunSumTests();
			}
			else if (choice == "2")
			{
				RunPrimeTests();
			}
			else if (choice == "3")
			{
				break;
			}
		}
	}

	static void RunSumTests()
	{
		Console.Write("\nEnter N : ");
		int N = int.Parse(Console.ReadLine());

		Console.Write("Enter number of workers: ");
		int workers = int.Parse(Console.ReadLine());

		Console.WriteLine($"\n=== Calculating sum from 1 to {N:N0} ===\n");

		Console.WriteLine("Running Single Process...");
		RunSingle(N);

		Console.WriteLine("\nRunning Process Pool...");
		RunProcessPool(N, workers);

		Console.WriteLine("\nRunning Thread Pool...");
		RunThreadPool(N, workers);
	}

	static void RunPrimeTests()
	{
		Console.Write("\nEnter range to find primes (start end): ");
		string[] range = Console.ReadLine().Split();
		int start = int.Parse(range[0]);
		int end = int.Parse(range[1]);

		Console.Write("Enter number of workers: ");
		int workers = int.Parse(Console.ReadLine());

		Console.WriteLine($"\n=== Calculating sum of primes from {start:N0} to {end:N0} ===\n");

		Console.WriteLine("Running Single Process...");
		RunSinglePrime(start, end);

		Console.WriteLine("\nRunning Process Pool...");
		RunProcessPoolPrime(start, end, workers);

		Console.WriteLine("\nRunning Thread Pool...");
		RunThreadPoolPrime(start, end, workers);
	}

	/* 
    ====== Single Process - Sum ======
     */
	static void RunSingle(int N)
	{
		var sw = Stopwatch.StartNew();
		long sum = 0;
		for (int i = 1; i <= N; i++)
		{
			sum += i;
		}
		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	/* 
    ====== Process Pool - Sum ======
     */
	static void RunProcessPool(int N, int workers)
	{
		var sw = Stopwatch.StartNew();
		int chunk = N / workers;
		long sum = 0;
		List<Process> processes = new List<Process>();

		for (int i = 0; i < workers; i++)
		{
			int start = i * chunk + 1;
			int end = (i == workers - 1) ? N : (i + 1) * chunk;

			var process = new Process();
			process.StartInfo.FileName = "./bin/Debug/net10.0/program";
			process.StartInfo.Arguments = $"worker {start} {end}";
			process.StartInfo.RedirectStandardOutput = true;
			process.StartInfo.UseShellExecute = false;
			process.Start();
			processes.Add(process);
		}

		foreach (var process in processes)
		{
			string output = process.StandardOutput.ReadToEnd();
			process.WaitForExit();
			sum += long.Parse(output.Trim());
		}

		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	/* 
    ====== Thread Pool - Sum ======
     */
	static void RunThreadPool(int N, int workers)
	{
		var sw = Stopwatch.StartNew();
		int chunk = N / workers;
		long[] results = new long[workers];

		Parallel.For(0, workers, new ParallelOptions { MaxDegreeOfParallelism = workers }, i =>
		{
			int start = i * chunk + 1;
			int end = (i == workers - 1) ? N : (i + 1) * chunk;

			long localSum = 0;
			for (int j = start; j <= end; j++)
			{
				localSum += j;
			}
			results[i] = localSum;
		});

		long sum = results.Sum();
		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	/* 
    ====== Single Process - Prime Sum ======
     */
	static void RunSinglePrime(int start, int end)
	{
		var sw = Stopwatch.StartNew();
		long sum = 0;
		int count = 0;

		for (int i = start; i <= end; i++)
		{
			if (IsPrime(i))
			{
				sum += i;
				count++;
			}
		}

		sw.Stop();
		Console.WriteLine($"Prime Count: {count}, Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	/* 
    ====== Process Pool - Prime Sum ======
     */
	static void RunProcessPoolPrime(int start, int end, int workers)
	{
		var sw = Stopwatch.StartNew();
		int range = end - start + 1;
		int chunk = range / workers;
		long sum = 0;
		List<Process> processes = new List<Process>();

		for (int i = 0; i < workers; i++)
		{
			int chunkStart = start + i * chunk;
			int chunkEnd = (i == workers - 1) ? end : start + (i + 1) * chunk - 1;

			var process = new Process();
			process.StartInfo.FileName = "./bin/Debug/net10.0/program";
			process.StartInfo.Arguments = $"prime-worker {chunkStart} {chunkEnd}";
			process.StartInfo.RedirectStandardOutput = true;
			process.StartInfo.UseShellExecute = false;
			process.Start();
			processes.Add(process);
		}

		foreach (var process in processes)
		{
			string output = process.StandardOutput.ReadToEnd();
			process.WaitForExit();
			sum += long.Parse(output.Trim());
		}

		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	/* 
    ====== Thread Pool - Prime Sum ======
     */
	static void RunThreadPoolPrime(int start, int end, int workers)
	{
		var sw = Stopwatch.StartNew();
		int range = end - start + 1;
		int chunk = range / workers;
		long[] results = new long[workers];

		Parallel.For(0, workers, new ParallelOptions { MaxDegreeOfParallelism = workers }, i =>
		{
			int chunkStart = start + i * chunk;
			int chunkEnd = (i == workers - 1) ? end : start + (i + 1) * chunk - 1;

			long localSum = 0;
			for (int j = chunkStart; j <= chunkEnd; j++)
			{
				if (IsPrime(j))
				{
					localSum += j;
				}
			}
			results[i] = localSum;
		});

		long sum = results.Sum();
		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	/* 
    ====== Helper: Check Prime ======
     */
	static bool IsPrime(int n)
	{
		if (n <= 1) return false;
		if (n <= 3) return true;
		if (n % 2 == 0 || n % 3 == 0) return false;

		for (int i = 5; i * i <= n; i += 6)
		{
			if (n % i == 0 || n % (i + 2) == 0)
				return false;
		}
		return true;
	}
}