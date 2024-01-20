import matplotlib.pyplot as plt
import numpy as np

def read_file(filename):
    data = []
    with open(filename, 'r') as file:
        for line in file:
            data.append(int(line))
    return data

def plot_cdf(data, title, x_label, y_label, save_filename):
    # sort the data in ascending order
    x = np.sort(data)
    x = np.log10(x)
    
    # get the cdf values of y
    N = len(data)
    y = np.arange(N) / float(N)
    
    # plotting
    plt.xlabel(x_label)
    plt.ylabel(y_label)
    plt.title(title)
    plt.plot(x, y, marker='o')
    
    # save the plot
    plt.savefig(save_filename)
    
    # clear the figure for the next plot
    plt.clf()

def main():
    # read and plot time.txt
    time_data = read_file("time.txt")
    plot_cdf(time_data, "Time CDF", "Time (log10)", "CDF", "time.png")

if __name__ == "__main__":
    main()
